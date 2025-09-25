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
    
    # Check if comprehensive test file exists
    if [ -f "./services/$service/${service}_comprehensive_test.go" ] || [ -f "./services/$service/*_comprehensive_test.go" ] 2>/dev/null; then
        print_status "Found comprehensive test suite for $service"
    fi
    
    if go test -v ./services/$service; then
        print_success "$service tests passed"
        return 0
    else
        print_error "$service tests failed"
        return 1
    fi
}

# Function to run comprehensive tests specifically
run_comprehensive_tests() {
    print_status "Running comprehensive test suites..."
    
    local services=("api" "collector" "msg_queue" "msg_queue_proxy" "streamer")
    local failed_services=()
    local test_files_found=0
    
    for service in "${services[@]}"; do
        local test_file_pattern="./services/$service/*_comprehensive_test.go"
        
        # Check if comprehensive test file exists
        if ls $test_file_pattern 1> /dev/null 2>&1; then
            test_files_found=$((test_files_found + 1))
            print_status "Running comprehensive tests for $service..."
            
            if go test -v -run "Test.*" ./services/$service/*_comprehensive_test.go ./services/$service/*.go 2>/dev/null || \
               go test -v ./services/$service; then
                print_success "✓ $service comprehensive tests passed"
            else
                print_error "✗ $service comprehensive tests failed"
                failed_services+=("$service")
            fi
        else
            print_warning "No comprehensive test file found for $service"
        fi
    done
    
    echo ""
    print_status "=== COMPREHENSIVE TEST SUMMARY ==="
    print_status "Test files found: $test_files_found/5"
    print_status "Services passed: $((5 - ${#failed_services[@]}))"
    print_status "Services failed: ${#failed_services[@]}"
    
    if [ ${#failed_services[@]} -eq 0 ]; then
        print_success "🎉 All comprehensive tests passed!"
        return 0
    else
        print_error "❌ Failed services: ${failed_services[*]}"
        return 1
    fi
}

# Function to run all tests
run_all_tests() {
    print_status "Running all tests..."
    
    # List of all services
    local services=("api" "collector" "msg_queue" "msg_queue_proxy" "streamer")
    local failed_services=()
    local total_tests=0
    local passed_tests=0
    
    print_status "Running comprehensive test suite for all 5 services..."
    
    # Run tests for each service individually to get detailed output
    for service in "${services[@]}"; do
        print_status "Testing $service service..."
        if go test -v ./services/$service 2>&1 | tee /tmp/${service}_test.log; then
            print_success "✓ $service service tests passed"
            passed_tests=$((passed_tests + 1))
        else
            print_error "✗ $service service tests failed"
            failed_services+=("$service")
        fi
        echo ""
    done
    
    # Run internal package tests
    print_status "Testing internal packages..."
    if go test -v ./internal/...; then
        print_success "✓ Internal package tests passed"
    else
        print_error "✗ Internal package tests failed"
        failed_services+=("internal")
    fi
    
    # Summary
    echo ""
    print_status "=== TEST SUMMARY ==="
    print_status "Services tested: ${#services[@]}"
    print_status "Services passed: $passed_tests"
    print_status "Services failed: ${#failed_services[@]}"
    
    if [ ${#failed_services[@]} -eq 0 ]; then
        print_success "🎉 All tests passed successfully!"
        return 0
    else
        print_error "❌ Failed services: ${failed_services[*]}"
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
    echo "  all             Run all tests (default)"
    echo "  api             Run API service tests only"
    echo "  collector       Run collector service tests only"
    echo "  msg_queue       Run message queue service tests only"
    echo "  msg_queue_proxy Run message queue proxy service tests only"
    echo "  streamer        Run streamer service tests only"
    echo "  services        Run all service tests (no internal packages)"
    echo "  coverage        Run tests with coverage report"
    echo "  race            Run tests with race detection"
    echo "  bench           Run benchmarks"
    echo "  lint            Run linter"
    echo "  clean           Clean test artifacts"
    echo "  verbose         Run all tests with extra verbose output"
    echo "  quick           Run basic tests without race detection"
    echo "  comprehensive   Run comprehensive test suites only"
    echo "  help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run all tests"
    echo "  $0 coverage           # Run tests with coverage"
    echo "  $0 api                # Run only API tests"
    echo "  $0 msg_queue_proxy    # Run only message queue proxy tests"
    echo "  $0 comprehensive      # Run comprehensive test suites"
    echo "  $0 services           # Run all service tests only"
    echo "  $0 race               # Run tests with race detection"
    echo "  $0 verbose            # Run with extra verbose output"
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
        "msg_queue_proxy")
            run_service_tests "msg_queue_proxy"
            exit_code=$?
            ;;
        "streamer")
            run_service_tests "streamer"
            exit_code=$?
            ;;
        "services")
            print_status "Running all service tests..."
            local services=("api" "collector" "msg_queue" "msg_queue_proxy" "streamer")
            local failed=0
            for service in "${services[@]}"; do
                if ! run_service_tests "$service"; then
                    failed=1
                fi
            done
            exit_code=$failed
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
        "verbose")
            print_status "Running verbose test suite..."
            go test -v -count=1 ./services/...
            exit_code=$?
            ;;
        "quick")
            print_status "Running quick test suite..."
            go test ./services/...
            exit_code=$?
            ;;
        "comprehensive")
            run_comprehensive_tests
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
