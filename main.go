// xdp_drop.go Drop incoming packets on XDP layer and count for which
// protocol type. Based on:
// https://github.com/iovisor/bcc/blob/master/examples/networking/xdp/xdp_drop_count.py
//
// Copyright (c) 2017 GustavoKatel
// Licensed under the Apache License, Version 2.0 (the "License")

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	bpf "github.com/iovisor/gobpf/bcc"
)

/*
#cgo CFLAGS: -I/usr/include/bcc/compat
#cgo LDFLAGS: -lbcc
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
void perf_reader_free(void *ptr);
*/
import "C"

func loadCModule(module string)(string, error){
	content, err := ioutil.ReadFile(module)
    if err != nil {
		log.Fatal(err)
		return "", err
    }

    // Convert []byte to string and print to screen
    return string(content), nil
}

func usage() {
	fmt.Printf("Usage: %v <ifdev>\n", os.Args[0])
	fmt.Printf("e.g.: %v eth0\n", os.Args[0])
	os.Exit(1)
}

func main() {
	var device string

	if len(os.Args) != 2 {
		usage()
	}

	device = os.Args[1]

	ret := "XDP_DROP"
	ctxtype := "xdp_md"

	source, err := loadCModule("xdp_drop.c")
	module := bpf.NewModule(source, []string{
		"-w",
		"-DRETURNCODE=" + ret,
		"-DCTXTYPE=" + ctxtype,
	})
	defer module.Close()

	fn, err := module.Load("bouncer", C.BPF_PROG_TYPE_XDP, 1, 65536)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load xdp prog: %v\n", err)
		os.Exit(1)
	}

	module.

	err = module.AttachXDP(device, fn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to attach xdp prog: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := module.RemoveXDP(device); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove XDP from %s: %v\n", device, err)
		}
	}()

	fmt.Println("Dropping packets, hit CTRL+C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	dropcnt := bpf.NewTable(module.TableId("dropcnt"), module)

	<-sig

	fmt.Printf("\n{IP protocol-number}: {total dropped pkts}\n")
	for it := dropcnt.Iter(); it.Next(); {
		key := bpf.GetHostByteOrder().Uint32(it.Key())
		value := bpf.GetHostByteOrder().Uint64(it.Leaf())

		if value > 0 {
			fmt.Printf("%v: %v pkts\n", key, value)
		}
	}
}