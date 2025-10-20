.PHONY: build test clean clean-examples help

# Default target
help:
	@echo "Peak - Generics for Salesforce Apex"
	@echo ""
	@echo "Available targets:"
	@echo "  make build          - Build the peak binary"
	@echo "  make test           - Run all tests"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make clean-examples - Remove generated .cls files from examples/"

# Build the peak binary
build:
	go build -o peak ./cmd/peak

# Run tests with coverage
test:
	go test ./... -v -cover

# Run tests with detailed coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -f peak
	rm -f coverage.out
	rm -f *.coverprofile

# Clean generated .cls files from examples directory
clean-examples:
	@echo "Removing generated .cls files from examples/..."
	@rm -f examples/*.cls
	@echo "Done."
