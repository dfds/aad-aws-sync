

test-coverage:
	go test ./... -coverprofile=cover.out

report:
	go tool cover -html=cover.out
