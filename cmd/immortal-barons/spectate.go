package main

import (
	"fmt"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// runSpectate steps the world forward one in-game day at a time and prints a
// compact per-day summary, then a final standings table (issue #42). It is a
// balance probe for AI play: a game of AI barons plays itself through
// DailyMaintenance, so watching the aggregates day by day shows whether realms
// expand, whether wars happen, and whether the planet collapses to one survivor.
//
// It does NOT reset the world — compose it with -reset-from-config for a clean
// run. Dates are synthesized forward from the world's own clock, so it needs no
// system-clock changes and no IB_GAME_DATE.
func runSpectate(cfg game.Config, days int) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}

	date, err := time.Parse("2006-01-02", w.LastMaintDate)
	if err != nil {
		// A world that has never had maintenance run has no clock yet; start from
		// the configured game start date so the first step is day one.
		if date, err = time.Parse("2006-01-02", cfg.GameStartDate); err != nil {
			date = time.Now()
		}
	}

	fmt.Printf("%-4s %5s %9s %11s %13s %9s %9s %7s %6s %5s\n",
		"day", "alive", "land", "people", "gold+bank", "offense", "defense", "treaty", "elim", "wars")
	prevAlive, prevBattles := len(w.AIEmpires()), w.BattlesTotal
	for i := 0; i < days; i++ {
		date = date.AddDate(0, 0, 1)
		w.DailyMaintenance(date.Format("2006-01-02"))

		s := spectateSnapshot(w)
		fmt.Printf("%-4d %5d %9d %11d %13d %9d %9d %7d %6d %5d\n",
			w.GameDay, s.alive, s.land, s.people, s.gold, s.offense, s.defense,
			len(w.Treaties), prevAlive-s.alive, w.BattlesTotal-prevBattles)
		prevAlive, prevBattles = s.alive, w.BattlesTotal
		if s.alive <= 1 {
			fmt.Printf("\nonly %d realm(s) left after %d day(s) — stopping early\n", s.alive, i+1)
			break
		}
	}

	fmt.Printf("\n%-18s %-10s %-6s %8s %10s %9s %8s %8s %6s %6s %6s %4s\n",
		"realm", "profile", "skill", "land", "networth", "score", "offense", "defense",
		"jets", "carr", "agents", "tax")
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		fmt.Printf("%-18.18s %-10s %-6s %8d %10d %9d %8d %8d %6d %6d %6d %4d\n",
			e.Name, e.AIProfile, e.AISkill, e.Land, w.NetWorth(e), e.Score,
			e.Offense(), e.Defense(), e.Jets, e.Carriers, e.Agents, e.Tax)
	}
	return store.Save(w, cfg)
}

// spectateSnapshot totals the living realms for one day's summary line.
type spectateTotals struct {
	alive, land, people, gold, offense, defense int
}

func spectateSnapshot(w *game.World) spectateTotals {
	var s spectateTotals
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		s.alive++
		s.land += e.Land
		s.people += e.People
		s.gold += e.Gold + e.Bank
		s.offense += e.Offense()
		s.defense += e.Defense()
	}
	return s
}
