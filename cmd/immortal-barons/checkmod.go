package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
)

// runCheckMod is -check-mod: it reads one mod folder with the same reader and
// limits a reset uses, and prints every value it changes from the base game.
// It checks the mod on its own; a reset also checks the mods it applies
// together, so two mods that pass here can still fail there as a pair.
//
// arg is a folder, or a mod's name as config.json lists it, which is looked up
// in the data directory's mods folder.
func runCheckMod(arg, dataDir string) error {
	path := arg
	if _, err := os.Stat(arg); err != nil && !strings.ContainsAny(arg, `/\`) {
		path = filepath.Join(dataDir, mod.Dir, arg)
	}
	m, r, err := mod.Check(path)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s (%s): OK\n", m.Ref().DisplayName(), m.Version, m.Hash)
	if m.Description != "" {
		fmt.Println(m.Description)
	}
	changes := mod.Diff(game.BaseRules(), r)
	if len(changes) == 0 {
		fmt.Println("It changes nothing from the base game.")
		return nil
	}
	fmt.Println("Changes from the base game:")
	for _, c := range changes {
		fmt.Println("  " + c)
	}
	return nil
}

// runPrintBaseMod is -print-base-mod: every value a mod can change, as the base
// game sets it, in the form a values.toml takes.
func runPrintBaseMod() {
	fmt.Print(mod.Values(game.BaseRules(), fmt.Sprintf(
		"Every value a mod can change, as Immortal Barons %s sets it.\n"+
			"Copy the sections you want into a mod's values.toml and change them.",
		game.VersionString())))
}
