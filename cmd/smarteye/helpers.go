package main

import "time"

func sleepMS(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
