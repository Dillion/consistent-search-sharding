package main

import (
	"log"
	"github.com/lafikl/consistent"
	"math/rand"
	"fmt"
)

func modifyLoad(workspaceLoad map[string]int64, min int64, max int64) {
	for key := range workspaceLoad {
		load := min + rand.Int63n(max-min+1) + workspaceLoad[key]
		if load < 0 {
			load = 0
		}
		workspaceLoad[key] = load
	}
}

func resetLoad(workspaceLoad map[string]int64) {
	for key := range workspaceLoad {
		workspaceLoad[key] = 0
	}
}

func setLoad(workspaceLoad map[string]int64, loadMax int64) {
	for key := range workspaceLoad {
		load := rand.Int63n(loadMax)
		workspaceLoad[key] = load
	}
}

func loadAboveThreshold(c *consistent.Consistent, currNodeToAssign string, threshold int64) bool {
	for node, load := range c.GetLoads() {
		if node == currNodeToAssign && load >= threshold {
			return true
		} else {
			return false
		}
	}
	return false
}

func printLoad(c *consistent.Consistent, desc string) {
	log.Println(desc)
	for node, load := range c.GetLoads() {
		fmt.Printf("%s: %d\n", node, load)
	}
}