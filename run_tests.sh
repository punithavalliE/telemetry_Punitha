#!/bin/bash

# Test runner script for the telemetry system
# This script runs all tests with various options

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to run tests for a specific service
run_service_tests() {
    local service=$1
    print_status "Running tests for $service service..."
    
    if go test -v ./services/$service; then
        print_success "$service tests passed"
        return 0
    else
        print_error "$service tests failed"
        return 1
    fi
}

# Function to run all tests
run_all_tests() {
    print_status "Running all tests..."
    
    if go test -v ./...; then
        print_success "All tests passed"
        return 0
    else
        print_error "Some tests failed"
        return 1
    fi
}

# Function to run tests with coverage
run_coverage_tests() {
    print_status "Running tests with coverage..."
    
    # Create coverage directory if it doesn't exist
    mkdir -p coverage
    
    # Run tests with coverage
    go test -coverprofile=coverage/coverage.out ./...
    
    if [ $? -eq 0 ]; then
        print_success "Coverage tests completed"
        
        # Generate HTML coverage report
        go tool cover -html=coverage/coverage.out -o coverage/coverage.html
        print_status "Coverage report generated: coverage/coverage.html"
        
        # Show coverage summary
        go tool cover -func=coverage/coverage.out | tail -1
        
        return 0
    else
        print_error "Coverage tests failed"
        return 1
    fi
}

# Function to run tests with race detection
run_race_tests() {
    print_status "Running tests with race detection..."
    
    if go test -race ./...; then
        print_success "Race detection tests passed"
        return 0
    else
        print_error "Race detection tests failed"
        return 1
    fi
}

# Function to run benchmarks
run_benchmarks() {
    print_status "Running benchmarks..."
    
    if go test -bench=. ./...; then
        print_success "Benchmarks completed"
        return 0
    else
        print_error "Benchmarks failed"
        return 1
    fi
}

# Function to check test dependencies
check_dependencies() {
    print_status "Checking test dependencies..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        return 1
    fi
    
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}')
    print_status "Using Go version: $GO_VERSION"
    
    # Check if we're in the right directory
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Please run this script from the project root directory."
        return 1
    fi
    
    print_success "All dependencies check passed"
    return 0
}

# Function to clean test artifacts
clean_artifacts() {
    print_status "Cleaning test artifacts..."
    
    # Remove coverage files
    rm -rf coverage/
    
    # Remove test binary files
    find . -name "*.test" -delete
    
    # Remove temporary test files (if any remain)
    find . -name "test-*" -delete 2>/dev/null || true
    
    print_success "Test artifacts cleaned"
}

# Function to lint the code
run_lint() {
    print_status "Running linter..."
    
    # Check if golangci-lint is installed
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run
        if [ $? -eq 0 ]; then
            print_success "Linting passed"
            return 0
        else
            print_error "Linting failed"
            return 1
        fi
    else
        print_warning "golangci-lint not found, skipping linting"
        return 0
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTION]"
    echo "Run tests for the telemetry system"
    echo ""
    echo "Options:"
    echo "  all         Run all tests (default)"
    echo "  api         Run API service tests only"
    echo "  collector   Run collector service tests only"
    echo "  msg_queue   Run message queue service tests only"
    echo "  streamer    Run streamer service tests only"
    echo "  coverage    Run tests with coverage report"
    echo "  race        Run tests with race detection"
    echo "  bench       Run benchmarks"
    echo "  lint        Run linter"
    echo "  clean       Clean test artifacts"
    echo "  help        Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0              # Run all tests"
    echo "  $0 coverage     # Run tests with coverage"
    echo "  $0 api          # Run only API tests"
    echo "  $0 race         # Run tests with race detection"
}

# Main script logic
main() {
    local option=${1:-all}
    local exit_code=0
    
    # Check dependencies first
    if ! check_dependencies; then
        exit 1
    fi
    
    case $option in
        "api")
            run_service_tests "api"
            exit_code=$?
            ;;
        "collector")
            run_service_tests "collector"
            exit_code=$?
            ;;
        "msg_queue")
            run_service_tests "msg_queue"
            exit_code=$?
            ;;
        "streamer")
            run_service_tests "streamer"
            exit_code=$?
            ;;
        "coverage")
            run_coverage_tests
            exit_code=$?
            ;;
        "race")
            run_race_tests
            exit_code=$?
            ;;
        "bench")
            run_benchmarks
            exit_code=$?
            ;;
        "lint")
            run_lint
            exit_code=$?
            ;;
        "clean")
            clean_artifacts
            exit_code=$?
            ;;
        "all")
            print_status "Starting comprehensive test suite..."
            
            # Run lint first
            run_lint
            
            # Run all tests
            run_all_tests
            test_result=$?
            
            # Run race detection
            run_race_tests
            race_result=$?
            
            # Run coverage
            run_coverage_tests
            coverage_result=$?
            
            # Determine overall result
            if [ $test_result -eq 0 ] && [ $race_result -eq 0 ] && [ $coverage_result -eq 0 ]; then
                print_success "All test suites passed!"
                exit_code=0
            else
                print_error "Some test suites failed"
                exit_code=1
            fi
            ;;
        "help"|"-h"|"--help")
            show_usage
            exit_code=0
            ;;
        *)
            print_error "Unknown option: $option"
            show_usage
            exit_code=1
            ;;
    esac
    
    exit $exit_code
}

# Run main function with all arguments
main "$@"
