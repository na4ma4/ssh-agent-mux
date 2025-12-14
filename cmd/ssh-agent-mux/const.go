package main

import "time"

const (
	retryInterval = 10 * time.Millisecond

	timeoutForSocketCreation = 5 * time.Second
)
