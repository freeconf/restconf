export YANGPATH=$(abspath ./yang)

test:
	go test -coverprofile test-coverage.out . ./...
	go tool cover -html=test-coverage.out -o test-coverage.html
	go tool cover -func test-coverage.out
