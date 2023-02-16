#!/bin/sh
go tool pprof -svg :8081/debug/pprof/heap > heap.svg
go tool pprof -svg :8081/debug/pprof/allocs > allocs.svg
