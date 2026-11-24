package main

import "flag"
import "fmt"
import "os"

import "go-mc-server/internal/storage"

func main() {
	var source = flag.String("source", "", "含 level.dat 的 PocketMine 世界目录")
	var dest = flag.String("dest", "", "本服新世界目录（上级目录须已存在）")
	flag.Parse()
	if *source == "" || *dest == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	var count, err = storage.ImportPMWorld(*source, *dest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "导入失败:", err)
		os.Exit(1)
	}
	fmt.Printf("已导入 %d 列到 %s\n", count, *dest)
}
