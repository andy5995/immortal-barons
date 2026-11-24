package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
	"github.com/andy5995/immortal-barons/internal/store"
)

// modArg reads a mod argument the same way in every command: anything with a
// slash is a path, and a bare name is a mod in dataDir's mods folder. Checking
// an existing mod also accepts a bare name that is a folder here; creating one
// must not, since the folder it names does not exist yet.
func modArg(arg, dataDir string, existing bool) string {
	if strings.ContainsAny(arg, `/\`) {
		return arg
	}
	if existing {
		if _, err := os.Stat(arg); err == nil {
			return arg
		}
	}
	return filepath.Join(dataDir, mod.Dir, arg)
}

// runCheckMod is -check-mod: it reads one mod folder with the same reader and
// limits a reset uses, and prints every value it changes from the base game.
// It checks the mod on its own; a reset also checks the mods it applies
// together, so two mods that pass here can still fail there as a pair.
//
// arg is a folder, or a mod's name as config.json lists it, which is looked up
// in the data directory's mods folder.
func runCheckMod(arg, dataDir string) error {
	m, r, err := mod.Check(modArg(arg, dataDir, true))
	if err != nil {
		return err
	}
	fmt.Printf("%s %s (%s): OK\n", m.Ref().DisplayName(), m.Version, m.Hash)
	if m.Description != "" {
		fmt.Println(m.Description)
	}
	fmt.Println("License: " + m.License)
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

// runReloadMods is -reload-mods: the game in dataDir plays the mods config.json
// lists, as their folders stand now, without a new game (store.ReloadMods).
func runReloadMods(cfg game.Config) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	// Loading warns that a mod folder has changed since the reset and that the
	// game keeps the old values, which is the one thing this run is about to
	// undo; a failed reload says so itself.
	mod.SetWarnOutput(io.Discard)
	w, err := store.Load(cfg)
	mod.SetWarnOutput(os.Stderr)
	if err != nil {
		return err
	}
	changes, err := store.ReloadMods(w, cfg)
	if err != nil {
		return err
	}
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Println("Mods reloaded; nothing changed.")
		return nil
	}
	fmt.Println("Mods reloaded. Changes:")
	for _, c := range changes {
		fmt.Println("  " + c)
	}
	return nil
}

// runInitMod is -init-mod: it creates an empty mod, which -check-mod accepts
// as it stands. arg is a folder, or a plain name, which is created in the data
// directory's mods folder as -check-mod looks it up.
func runInitMod(arg, dataDir string) error {
	dir := modArg(arg, dataDir, false)
	if err := mod.Init(dir); err != nil {
		return err
	}
	fmt.Printf("Created %s with %s and %s.\n", dir, mod.ManifestFile, mod.ValuesFile)
	fmt.Printf("Its license is %s; change it in %s if you want another, and add the\nlicense's text as a LICENSE file before you share the mod.\n", mod.DefaultLicense, mod.ManifestFile)
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
