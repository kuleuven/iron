package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/cmd/iron/tabwriter"
)

type Printer interface {
	Setup(hasACL, hasMeta bool)
	Print(name string, i api.Record)
	Flush()
}

type TablePrinter struct {
	Writer *tabwriter.TabWriter
	Zone   string
	Items  int
}

func (tp *TablePrinter) Setup(hasACL, hasMeta bool) {
	header1 := "CREATOR\tSIZE\tDATE\tSTATUS\tNAME"

	if hasMeta {
		header1 += "\t─── METADATA KEY\tVALUE\tUNITS\n"
	} else {
		header1 += "\t\t\n"
	}

	var header2 string

	if hasACL {
		header2 = " └─ USER\tACCESS\n"
	}

	fmt.Fprintf(tp.Writer, "%s%s%s%s", Bold, header1, header2, Reset)
}

func (tp *TablePrinter) Print(name string, i api.Record) { //nolint:funlen
	t := i.ModTime().Format("Jan 01  2006")

	if i.ModTime().Year() == time.Now().Year() {
		t = i.ModTime().Format("Jan 01 15:04")
	}

	size := humanize.Bytes(uint64(i.Size()))

	var status, owner, color string

	switch v := i.Sys().(type) {
	case *api.DataObject:
		for _, r := range v.Replicas {
			status = appendStatus(status, r.Status)
			owner = tp.formatUser(r.Owner, r.OwnerZone, false)
			color = NoColor
		}

	case *api.Collection:
		if v.Inheritance {
			status = "+"
		}

		name += "/"
		owner = tp.formatUser(v.Owner, v.OwnerZone, false)
		color = Green
	}

	var acl []string

	for p, a := range i.Access() {
		acl = append(acl, fmt.Sprintf("%s %s%s\t%s%s",
			bracket(p+1, len(i.Access())+1),
			Cyan,
			tp.formatUser(a.User.Name, a.User.Zone, a.User.Type == "rodsgroup"),
			formatPermission(a.Permission),
			NoColor,
		))
	}

	var meta []string

	for p, m := range i.Metadata() {
		meta = append(meta, fmt.Sprintf("%s %s%s\t%s\t%s%s",
			bracket(p, len(i.Metadata())),
			Yellow,
			m.Name,
			m.Value,
			m.Units,
			NoColor,
		))
	}

	header := fmt.Sprintf("%s\t%s\t%s\t%s\t%s%s%s",
		owner,
		size,
		t,
		status,
		color+Bold,
		name,
		NoColor+NoBold,
	)

	if len(meta) > 0 {
		header += "\t" + meta[0] + "\n"

		meta = meta[1:]
	} else {
		header += "\n"
	}

	fmt.Fprint(tp.Writer, header)

	for i := range max(len(acl), len(meta)) {
		aclLine := "\t"
		metaLine := "\t\t"

		if i < len(acl) {
			aclLine = acl[i]
		}

		if i < len(meta) {
			metaLine = meta[i]
		}

		fmt.Fprintf(tp.Writer, "%s\t\t\t\t%s\n", aclLine, metaLine)
	}
}

func appendStatus(list, status string) string {
	switch status {
	case "1":
		return list + "✔" // Good replica
	case "0":
		return list + "✘" // Stale replica
	case "2":
		return list + "🔒" // Write locked
	case "4":
		return list + "🚫" // Intermediate
	default:
		return list + status
	}
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
	Writer io.Writer
}

func (jp *JSONPrinter) Setup(asACL, hasMeta bool) {
	// empty
}

func (jp *JSONPrinter) Print(name string, i api.Record) {
	json.NewEncoder(jp.Writer).Encode(toMap(name, i)) //nolint:errcheck,errchkjson
}

func (jp *JSONPrinter) Flush() {
	// empty
}

func toMap(name string, i api.Record) map[string]any {
	var (
		creator string
		id      int64
	)

	switch v := i.Sys().(type) {
	case *api.DataObject:
		id = v.ID
		creator = v.Replicas[0].Owner

	case *api.Collection:
		id = v.ID
		creator = v.Owner
	}

	return map[string]any{
		"name":     name,
		"size":     i.Size(),
		"modified": i.ModTime().Format(time.RFC3339),
		"creator":  creator,
		"id":       id,
		"acl":      i.Access(),
		"metadata": i.Metadata(),
	}
}
