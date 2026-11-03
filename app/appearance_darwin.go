package main

// #cgo LDFLAGS: -framework Cocoa
// void omdUseSidebarMaterial(void);
import "C"

func configureWindowAppearance() { C.omdUseSidebarMaterial() }
