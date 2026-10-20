package main

import (
	"fmt"
	"os"
	"path/filepath"

	"arsivacici/arc"
)

// usage: arctest <archive> <password> <outdir>
func main() {
	a, err := arc.Open(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Println("OPEN ERR:", err)
		os.Exit(1)
	}
	defer a.Close()
	fmt.Printf("%s  %d entries\n", a.Format(), len(a.Entries()))
	for _, e := range a.Entries() {
		fmt.Printf("  %-40q dir=%v enc=%v link=%v size=%d mod=%s\n", e.Path, e.IsDir, e.Encrypted, e.IsLink, e.Size, e.Modified.Format("2006-01-02"))
	}
	if err := arc.Test(a, nil); err != nil {
		fmt.Println("TEST ERR:", err)
	} else {
		fmt.Println("TEST OK")
	}
	out := filepath.Join(os.Args[3], filepath.Base(os.Args[1]))
	os.RemoveAll(out)
	res, err := arc.Extract(a, arc.ExtractOptions{Dest: out})
	fmt.Printf("EXTRACT files=%d skipped=%d unsafe=%v err=%v\n", res.Files, res.Skipped, res.Unsafe, err)
}
