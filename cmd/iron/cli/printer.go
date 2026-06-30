package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kuleuven/iron/api"
)

// Main row column headers.
const (
	colCreator  = "CREATOR"
	colSize     = "SIZE"
	colDate     = "DATE"
	colStatus   = "STATUS"
	colChecksum = "CHECKSUM"
	colName     = "NAME"
)

// Subtable section labels.
const (
	sectionMeta    = "META"
	sectionACL     = "ACL"
	sectionReplica = "REPL"
)

// Subtable column headers.
const (
	colMetaKey   = "KEY"
	colMetaValue = "VALUE"
	colMetaUnits = "UNITS"

	colACLUser   = "USER"
	colACLAccess = "ACCESS"

	colReplicaNum      = "REPLICA"
	colReplicaStatus   = "STATUS"
	colReplicaChecksum = "CHECKSUM"
	colReplicaResource = "RESOURCE"
)

// rodsGroupType is the iRODS user type for groups.
const rodsGroupType = "rodsgroup"

// subtableIndent skips the six main row columns (CREATOR through NAME) so that
// subtable content appears to the right of the name column. Columns hidden by
// the writer collapse to nothing, so the indent stays correct even when the
// STATUS and CHECKSUM columns are hidden in replica mode.
const subtableIndent = "\t\t\t\t\t\t"

type Printer interface {
	Setup(hasACL, hasMeta, hasCollectionSize, hasReplicas bool)
	Print(name string, i api.Record)
	Flush()
}

type TablePrinter struct {
	Writer interface {
		io.Writer
		Flush() error
	}
	Zone string

	hasACL             bool
	hasMeta            bool
	hasCollectionSizes bool
	hasReplicas        bool

	// inlineSubtableHeader is true when exactly one subtable is requested, in
	// which case its column headers are appended to the main header row instead
	// of being repeated as a section header above each record's subtable.
	inlineSubtableHeader bool

	// firstRecord tracks whether any record has been printed yet, used to emit
	// an inter-object separator when subtables are active.
	firstRecord bool
}

func (tp *TablePrinter) Setup(hasACL, hasMeta, hasCollectionSizes, hasReplicas bool) {
	tp.hasACL = hasACL
	tp.hasMeta = hasMeta
	tp.hasCollectionSizes = hasCollectionSizes
	tp.hasReplicas = hasReplicas
	tp.inlineSubtableHeader = countTrue(hasACL, hasMeta, hasReplicas) == 1
	tp.firstRecord = true

	header := strings.Join([]string{colCreator, colSize, colDate, colStatus, colChecksum, colName}, "\t")

	if tp.inlineSubtableHeader {
		header += "\t" + tp.subtableHeader()
	}

	fmt.Fprintf(tp.Writer, "%s%s\n%s", Bold, header, Reset)
}

// countTrue returns the number of true values among the given booleans.
func countTrue(bs ...bool) int {
	n := 0

	for _, b := range bs {
		if b {
			n++
		}
	}

	return n
}

// subtableHeader returns the section label followed by the column headers of the
// single active subtable, joined by tabs. It is only meaningful when exactly
// one subtable is requested.
func (tp *TablePrinter) subtableHeader() string {
	switch {
	case tp.hasReplicas:
		return strings.Join([]string{sectionReplica, colReplicaNum, colReplicaResource, colReplicaStatus, colReplicaChecksum}, "\t")
	case tp.hasMeta:
		return strings.Join([]string{sectionMeta, colMetaKey, colMetaValue, colMetaUnits}, "\t")
	case tp.hasACL:
		return strings.Join([]string{sectionACL, colACLUser, colACLAccess}, "\t")
	default:
		return ""
	}
}

func (tp *TablePrinter) Print(name string, i api.Record) {
	// Emit a blank line between objects when multiple subtables are active so
	// that multi-row records don't visually merge into one another.
	if !tp.firstRecord && countTrue(tp.hasACL, tp.hasMeta, tp.hasReplicas) > 1 {
		fmt.Fprintln(tp.Writer)
	}

	tp.firstRecord = false

	t := i.ModTime().Format("Jan 02  2006")

	if i.ModTime().Year() == time.Now().Year() {
		t = i.ModTime().Format("Jan 02 15:04")
	}

	var status, owner, checksum, color string

	switch v := i.Sys().(type) {
	case *api.DataObject:
		for _, r := range v.Replicas {
			status = appendStatus(status, r.Status)
			owner = tp.formatUser(r.Owner, r.OwnerZone, false)
			checksum = parseIrodsChecksum(r.Checksum)
		}

		color = NoColor

	case *api.Collection:
		if v.Inheritance {
			status = "+"
		}

		name += "/"
		owner = tp.formatUser(v.Owner, v.OwnerZone, false)
		color = Green
	}

	mainRowEnd := "\n"
	if tp.inlineSubtableHeader {
		// In inline mode the first subtable value row continues on the main row,
		// so the main row must not be terminated with a newline yet.
		mainRowEnd = ""
	}

	fmt.Fprintf(tp.Writer, "%s\t%s\t%s\t%s\t%s\t%s%s%s%s",
		owner,
		tp.formatSize(i),
		t,
		status,
		checksum,
		color+Bold, name, NoColor+NoBold,
		mainRowEnd,
	)

	tp.printSubtables(i)
}

// printSubtables renders the requested subtables beneath an item's main row.
//
// In inline mode (a single subtable) the first value row continues the main row,
// which Print therefore leaves unterminated. In multi-subtable mode each subtable
// is rendered with its own section header and consecutive subtables are separated
// by a blank line.
func (tp *TablePrinter) printSubtables(i api.Record) {
	first := true

	if tp.hasReplicas {
		if obj, ok := i.Sys().(*api.DataObject); ok {
			first = tp.printReplicaSubtable(obj.Replicas, first)
		}
	}

	if tp.hasMeta {
		first = tp.printMetaSubtable(i.Metadata(), first)
	}

	if tp.hasACL {
		first = tp.printACLSubtable(i.Access(), first)
	}

	// Inline mode left the main row unterminated; if no subtable produced output
	// to continue it (e.g. a collection or an empty subtable), terminate it now.
	if tp.inlineSubtableHeader && first {
		fmt.Fprintln(tp.Writer)
	}
}

// subtableRowLead returns the leading whitespace for the value row at index idx.
// The first row of the first subtable in inline mode continues the main row (a
// single tab into the section column); every other row is indented past the six
// main columns.
func (tp *TablePrinter) subtableRowLead(idx int, first bool) string {
	if idx == 0 && first && tp.inlineSubtableHeader {
		return "\t"
	}

	return subtableIndent
}

// subtableSeparator emits a blank line before a subtable when an earlier subtable
// has already been printed, keeping multiple subtables visually distinct.
func (tp *TablePrinter) subtableSeparator(first bool) {
	if !first {
		fmt.Fprintln(tp.Writer)
	}
}

// printReplicaSubtable renders per-replica details beneath a data object's main row.
// Each replica occupies its own row showing replica number, status, checksum and resource.
// It reports whether the subtable is still the first to be printed (true when empty).
func (tp *TablePrinter) printReplicaSubtable(replicas []api.Replica, first bool) bool {
	if len(replicas) == 0 {
		return first
	}

	tp.subtableSeparator(first)

	// Section label + column headers (cols 6–10).
	if !tp.inlineSubtableHeader {
		fmt.Fprintf(tp.Writer, "%s%s%s%s\t%s\t%s\t%s\t%s\n",
			subtableIndent,
			Bold, sectionReplica, NoBold,
			colReplicaNum, colReplicaResource, colReplicaStatus, colReplicaChecksum,
		)
	}

	for idx, r := range replicas {
		fmt.Fprintf(tp.Writer, "%s\t%d\t%s\t%s\t%s\n",
			tp.subtableRowLead(idx, first),
			r.Number,
			r.ResourceHierarchy,
			statusIcon(r.Status),
			parseIrodsChecksum(r.Checksum),
		)
	}

	return false
}

// printMetaSubtable renders metadata attributes beneath an item's main row.
// Each attribute occupies its own row showing name, value and units.
// It reports whether the subtable is still the first to be printed (true when empty).
func (tp *TablePrinter) printMetaSubtable(meta []api.Metadata, first bool) bool {
	if len(meta) == 0 {
		return first
	}

	tp.subtableSeparator(first)

	// Section label + column headers (cols 6–9).
	if !tp.inlineSubtableHeader {
		fmt.Fprintf(tp.Writer, "%s%s%s%s\t%s\t%s\t%s\n",
			subtableIndent,
			Bold, sectionMeta, NoBold,
			colMetaKey, colMetaValue, colMetaUnits,
		)
	}

	for idx, m := range meta {
		fmt.Fprintf(tp.Writer, "%s\t%s%s%s\t%s\t%s\n",
			tp.subtableRowLead(idx, first),
			Yellow, m.Name, NoColor,
			m.Value,
			m.Units,
		)
	}

	return false
}

// printACLSubtable renders access control entries beneath an item's main row.
// Each entry occupies its own row showing the user (or group) and their permission.
// It reports whether the subtable is still the first to be printed (true when empty).
func (tp *TablePrinter) printACLSubtable(acl []api.Access, first bool) bool {
	if len(acl) == 0 {
		return first
	}

	tp.subtableSeparator(first)

	// Section label + column headers (cols 6–8).
	if !tp.inlineSubtableHeader {
		fmt.Fprintf(tp.Writer, "%s%s%s%s\t%s\t%s\n",
			subtableIndent,
			Bold, sectionACL, NoBold,
			colACLUser, colACLAccess,
		)
	}

	for idx, a := range acl {
		user := tp.formatUser(a.User.Name, a.User.Zone, a.User.Type == rodsGroupType)

		fmt.Fprintf(tp.Writer, "%s\t%s%s%s\t%s\n",
			tp.subtableRowLead(idx, first),
			Cyan, user, NoColor,
			formatPermission(a.Permission),
		)
	}

	return false
}

func (tp *TablePrinter) formatSize(i api.Record) string {
	if i.IsDir() && !tp.hasCollectionSizes {
		return ""
	}

	return humanize.Bytes(uint64(i.Size()))
}

func statusIcon(status string) string {
	switch status {
	case "1":
		return "✔" // Good replica
	case "0":
		return "✘" // Stale replica
	case "2":
		return "⚿" // Write locked
	case "4":
		return "…" // Intermediate
	default:
		return status
	}
}

func appendStatus(list, status string) string {
	return list + statusIcon(status) + " "
}

func bracket(i, n int) string {
	switch {
	case n == 1:
		return "───"
	case i == 0:
		return "─┬─"
	case i+1 == n:
		return " └─"
	default:
		return " ├─"
	}
}

var (
	Reset     = "\033[00m"
	Red       = "\033[31m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Magenta   = "\033[35m"
	Cyan      = "\033[36m"
	Gray      = "\033[37m"
	LightGray = "\033[38m"
	White     = "\033[97m"
	NoColor   = "\033[39m"
	Bold      = "\033[01m"
	NoBold    = "\033[22m"

	HeaderBackground = "\033[48;5;6m"
	RowBackground    = "\033[48;5;14m"
	AltRowBackground = "\033[48;5;111m" // 153,111  81,117
	NoBackground     = "\033[49m"

	Underline   = "\033[4m"
	NoUnderline = "\033[24m"
)

func (tp *TablePrinter) formatUser(name, zone string, isGroup bool) string {
	if isGroup {
		name = fmt.Sprintf("g:%s", name)
	}

	if zone == tp.Zone {
		return name
	}

	return fmt.Sprintf("%s#%s", name, zone)
}

func formatPermission(p string) string {
	switch p {
	case "own":
		return "own"
	case "read object", "read_object":
		return "read"
	case "write object", "write_object", "modify_object":
		return "write"
	case "delete_object":
		return "delete"
	default:
		return p
	}
}

func (tp *TablePrinter) Flush() {
	tp.Writer.Flush()
}

type JSONPrinter struct {
	Writer                       io.Writer
	hasACL, hasMeta, hasReplicas bool
}

func (jp *JSONPrinter) Setup(hasACL, hasMeta, _, hasReplicas bool) {
	jp.hasACL, jp.hasMeta, jp.hasReplicas = hasACL, hasMeta, hasReplicas
}

func (jp *JSONPrinter) Print(name string, i api.Record) {
	m := toMap(name, i)

	if !jp.hasACL {
		delete(m, jsonFieldACL)
	}

	if !jp.hasMeta {
		delete(m, jsonFieldMetadata)
	}

	if !jp.hasReplicas {
		delete(m, jsonFieldReplicas)
	}

	json.NewEncoder(jp.Writer).Encode(m) //nolint:errcheck,errchkjson
}

func (jp *JSONPrinter) Flush() {
	// empty
}

// JSON field names used in the JSONPrinter output.
const (
	jsonFieldName     = "name"
	jsonFieldSize     = "size"
	jsonFieldModified = "modified"
	jsonFieldCreator  = "creator"
	jsonFieldID       = "id"
	jsonFieldACL      = "acl"
	jsonFieldMetadata = "metadata"
	jsonFieldReplicas = "replicas"
	jsonFieldChecksum = "checksum"
	jsonFieldNumber   = "number"
	jsonFieldResource = "resource"
	jsonFieldStatus   = "status"
)

func toMap(name string, i api.Record) map[string]any {
	var (
		creator  string
		checksum *string
		id       int64
		replicas []map[string]any
	)

	switch v := i.Sys().(type) {
	case *api.DataObject:
		id = v.ID
		creator = v.Replicas[0].Owner

		str := parseIrodsChecksum(v.Replicas[0].Checksum)
		checksum = &str

		for _, r := range v.Replicas {
			replicas = append(replicas, map[string]any{
				jsonFieldNumber:   r.Number,
				jsonFieldResource: r.ResourceHierarchy,
				jsonFieldStatus:   r.Status,
				jsonFieldChecksum: parseIrodsChecksum(r.Checksum),
			})
		}

	case *api.Collection:
		id = v.ID
		creator = v.Owner
	}

	m := map[string]any{
		jsonFieldName:     name,
		jsonFieldSize:     i.Size(),
		jsonFieldModified: i.ModTime().Format(time.RFC3339),
		jsonFieldCreator:  creator,
		jsonFieldID:       id,
		jsonFieldACL:      i.Access(),
		jsonFieldMetadata: i.Metadata(),
		jsonFieldReplicas: replicas,
	}

	if checksum != nil {
		m[jsonFieldChecksum] = *checksum
	}

	return m
}

func parseIrodsChecksum(s string) string {
	if s == "" {
		return ""
	}

	if chs, err := api.ParseIrodsChecksum(s); err == nil {
		return fmt.Sprintf("%0x", chs)
	}

	return ""
}
