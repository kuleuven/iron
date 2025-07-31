package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kuleuven/iron/api"
)

type Printer interface {
	Flush()
	Print(name string, i api.Record)
}

type TablePrinter struct {
	Writer *tabwriter.Writer
	Zone   string
}

func (tp *TablePrinter) Print(name string, i api.Record) { //nolint:funlen
	if i.IsDir() {
		name += "/"
	}

	t := i.ModTime().Format("Jan 01  2006")

	if i.ModTime().Year() == time.Now().Year() {
		t = i.ModTime().Format("Jan 01 15:04")
	}

	size := humanize.Bytes(uint64(i.Size()))

	var status, owner string

	switch v := i.Sys().(type) {
	case *api.DataObject:
		for _, r := range v.Replicas {
			status += r.Status
			owner = tp.formatUser(r.Owner, r.OwnerZone)
		}

	case *api.Collection:
		if v.Inheritance {
			status = "+"
		}

		owner = tp.formatUser(v.Owner, v.OwnerZone)
	}

	fmt.Fprintf(tp.Writer, "%s\t%s\t%s\t%s\t%s",
		owner,
		size,
		t,
		status,
		name,
	)

	var acl []string

	for _, a := range i.Access() {
		acl = append(acl, fmt.Sprintf("%s\t%s", tp.formatUser(a.User.Name, a.User.Zone), formatPermission(a.Permission)))
	}

	var meta []string

	for _, m := range i.Metadata() {
		meta = append(meta, fmt.Sprintf("┊\t%s\t%s\t%s", m.Name, m.Value, m.Units))
	}

	slices.Sort(acl)
	slices.Sort(meta)

	for i := range max(len(acl), len(meta)) {
		aclLine := "\t"
		metaLine := "\t\t\t"

		if i < len(acl) {
			aclLine = acl[i]
		}

		if i < len(meta) {
			metaLine = meta[i]
		}

		fmt.Fprintf(tp.Writer, "%s\n%s\t\t%s", Cyan, aclLine, metaLine)
	}

	fmt.Fprintf(tp.Writer, "%s\n", Reset)
}

var (
	Reset   = "\033[00m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[37m"
	White   = "\033[97m"
)

func (tp *TablePrinter) formatUser(name, zone string) string {
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

func (jp *JSONPrinter) Print(name string, i api.Record) {
	json.NewEncoder(jp.Writer).Encode(toMap(name, i)) //nolint:errcheck,errchkjson
}

func (jp *JSONPrinter) Flush() {
	// empty
}

type ListAppender struct {
	List []map[string]interface{}
	sync.Mutex
}

func (la *ListAppender) Print(name string, i api.Record) {
	la.Lock()

	defer la.Unlock()

	la.List = append(la.List, toMap(name, i))
}

func (la *ListAppender) Flush() {
	// empty
}

func toMap(name string, i api.Record) map[string]interface{} {
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

	return map[string]interface{}{
		"name":     name,
		"size":     i.Size(),
		"modified": i.ModTime().Format(time.RFC3339),
		"creator":  creator,
		"id":       id,
		"acl":      i.Access(),
		"metadata": i.Metadata(),
	}
}
