package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
