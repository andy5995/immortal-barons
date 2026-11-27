package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/modsim"
)

// modOptionFlag collects each -mod-option id=value as text. The mod declares
// each option's type, so modsim checks the value once the mod is read.
type modOptionFlag map[string]string

func modOptionVar(lang string) *modOptionFlag {
	m := modOptionFlag{}
	flag.Var(&m, "mod-option", i18n.T(lang, "with -sim-mod, play the mod with its option `<id>=<value>` set; repeat it for more options"))
	return &m
}

func (m *modOptionFlag) String() string {
	if m == nil || len(*m) == 0 {
		return ""
	}
	return fmt.Sprint(map[string]string(*m))
}

func (m *modOptionFlag) Set(s string) error {
	id, val, ok := strings.Cut(s, "=")
	if !ok || id == "" || val == "" {
		return fmt.Errorf("want <id>=<value>, not %q", s)
	}
	if _, dup := (*m)[id]; dup {
		return fmt.Errorf("option %s is given twice", id)
	}
	(*m)[id] = val
	return nil
}

// runSimMod is -sim-mod: it plays the mod and the base game side by side in
// memory (internal/modsim) and prints how they differ. It reads the mod folder and no game, and
// writes only the CSV file -sim-csv names.
func runSimMod(o *opts) error {
	if flag.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", flag.Arg(0))
	}
	rep, err := modsim.Run(modsim.Options{
		ModPath: modArg(*o.simMod, *o.dataDir, true),
		Choices: *o.modOptions,
		Days:    *o.simDays,
		Seeds:   *o.simSeeds,
		Realms:  *o.simRealms,
	})
	if err != nil {
		return err
	}
	fmt.Print(rep.Text())
	if *o.simCSV != "" {
		if err := os.WriteFile(*o.simCSV, []byte(rep.CSV()), 0o644); err != nil {
			return err
		}
		fmt.Println("Wrote " + *o.simCSV)
	}
	return nil
}

// checkSimCompanions refuses a flag that only -sim-mod reads when -sim-mod is
// not given, so a mistyped command is not silently ignored.
func checkSimCompanions() error {
	var stray []string
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "sim-days", "sim-seeds", "sim-realms", "sim-csv", "mod-option":
			stray = append(stray, "-"+f.Name)
		}
	})
	if len(stray) > 0 {
		return fmt.Errorf("%s only works with -sim-mod", strings.Join(stray, ", "))
	}
	return nil
}
