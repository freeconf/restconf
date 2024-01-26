# export YANGPATH=$(abspath ./yang)

test:
	go test -coverprofile test-coverage.out . ./...
	go tool cover -html=test-coverage.out -o test-coverage.html
	go tool cover -func test-coverage.out

TARGET_DOCS = \
  ietf-subscribed-notifications

DOCS_OUT = \
	$(foreach F,$(TARGET_DOCS),docs/$(F).html)

.PHONY: docs
docs: $(DOCS_OUT)

$(DOCS_OUT) : docs/%.html :
	go run github.com/freeconf/yang/cmd/fc-yang doc \
		-f dot \
		-on replay \
		-on configured \
		-ypath yang/ietf-rfc \
		-module $* > docs/$*.dot
	dot -Tsvg docs/$*.dot -o docs/$*.svg	
	go run github.com/freeconf/yang/cmd/fc-yang doc \
		-f html \
		-on replay \
		-on configured \
		-ypath yang/ietf-rfc \
		-module $* > $@
