# PowerShell test runner script for the telemetry system
param(
    [Parameter(Position=0)]
    [ValidateSet("all", "api", "collector", "msg_queue", "streamer", "coverage", "race", "bench", "lint", "clean", "help")]
    [string]$Option = "all"
)

# Function to print colored output
function Write-Status {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Function to run tests for a specific service
function Run-ServiceTests {
    param([string]$Service)
    
    Write-Status "Running tests for $Service service..."
    
    $result = & go test -v ./services/$Service
    if ($LASTEXITCODE -eq 0) {
        Write-Success "$Service tests passed"
        return $true
    } else {
        Write-Error "$Service tests failed"
        return $false
    }
}

# Function to run all tests
function Run-AllTests {
    Write-Status "Running all tests..."
    
    $result = & go test -v ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Success "All tests passed"
        return $true
    } else {
        Write-Error "Some tests failed"
        return $false
    }
}

# Function to run tests with coverage
function Run-CoverageTests {
    Write-Status "Running tests with coverage..."
    
    # Create coverage directory if it doesn't exist
    if (!(Test-Path "coverage")) {
        New-Item -ItemType Directory -Path "coverage" | Out-Null
    }
    
    # Run tests with coverage
    & go test -coverprofile=coverage/coverage.out ./...
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Coverage tests completed"
        
        # Generate HTML coverage report
        & go tool cover -html=coverage/coverage.out -o coverage/coverage.html
        Write-Status "Coverage report generated: coverage/coverage.html"
        
        # Show coverage summary
        $coverageOutput = & go tool cover -func=coverage/coverage.out
        $lastLine = $coverageOutput | Select-Object -Last 1
        Write-Host $lastLine
        
        return $true
    } else {
        Write-Error "Coverage tests failed"
        return $false
    }
}

# Function to run tests with race detection
function Run-RaceTests {
    Write-Status "Running tests with race detection..."
    
    $result = & go test -race ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Race detection tests passed"
        return $true
    } else {
        Write-Error "Race detection tests failed"
        return $false
    }
}

# Function to run benchmarks
function Run-Benchmarks {
    Write-Status "Running benchmarks..."
    
    $result = & go test -bench=. ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Benchmarks completed"
        return $true
    } else {
        Write-Error "Benchmarks failed"
        return $false
    }
}

# Function to check test dependencies
function Test-Dependencies {
    Write-Status "Checking test dependencies..."
    
    # Check if Go is installed
    try {
        $goVersion = & go version 2>$null
        Write-Status "Using Go version: $($goVersion -replace 'go version ', '')"
    } catch {
        Write-Error "Go is not installed or not in PATH"
        return $false
    }
    
    # Check if we're in the right directory
    if (!(Test-Path "go.mod")) {
        Write-Error "go.mod not found. Please run this script from the project root directory."
        return $false
    }
    
    Write-Success "All dependencies check passed"
    return $true
}

# Function to clean test artifacts
function Clear-TestArtifacts {
    Write-Status "Cleaning test artifacts..."
    
    # Remove coverage files
    if (Test-Path "coverage") {
        Remove-Item -Path "coverage" -Recurse -Force
    }
    
    # Remove test binary files
    Get-ChildItem -Path . -Recurse -Name "*.test" | Remove-Item -Force
    
    # Remove temporary test files (if any remain)
    Get-ChildItem -Path . -Recurse -Name "test-*" -ErrorAction SilentlyContinue | Remove-Item -Force
    
    Write-Success "Test artifacts cleaned"
}

# Function to run linter
function Invoke-Lint {
    Write-Status "Running linter..."
    
    # Check if golangci-lint is installed
    try {
        & golangci-lint --version 2>$null | Out-Null
        $result = & golangci-lint run
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Linting passed"
            return $true
        } else {
            Write-Error "Linting failed"
            return $false
        }
    } catch {
        Write-Warning "golangci-lint not found, skipping linting"
        return $true
    }
}

# Function to show usage
function Show-Usage {
    Write-Host "Usage: .\run_tests.ps1 [OPTION]"
    Write-Host "Run tests for the telemetry system"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  all         Run all tests (default)"
    Write-Host "  api         Run API service tests only"
    Write-Host "  collector   Run collector service tests only"
    Write-Host "  msg_queue   Run message queue service tests only"
    Write-Host "  streamer    Run streamer service tests only"
    Write-Host "  coverage    Run tests with coverage report"
    Write-Host "  race        Run tests with race detection"
    Write-Host "  bench       Run benchmarks"
    Write-Host "  lint        Run linter"
    Write-Host "  clean       Clean test artifacts"
    Write-Host "  help        Show this help message"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\run_tests.ps1              # Run all tests"
    Write-Host "  .\run_tests.ps1 coverage     # Run tests with coverage"
    Write-Host "  .\run_tests.ps1 api          # Run only API tests"
    Write-Host "  .\run_tests.ps1 race         # Run tests with race detection"
}

# Main script logic
function Main {
    # Check dependencies first
    if (!(Test-Dependencies)) {
        exit 1
    }
    
    $exitCode = 0
    
    switch ($Option) {
        "api" {
            $result = Run-ServiceTests "api"
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "collector" {
            $result = Run-ServiceTests "collector"
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "msg_queue" {
            $result = Run-ServiceTests "msg_queue"
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "streamer" {
            $result = Run-ServiceTests "streamer"
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "coverage" {
            $result = Run-CoverageTests
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "race" {
            $result = Run-RaceTests
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "bench" {
            $result = Run-Benchmarks
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "lint" {
            $result = Invoke-Lint
            $exitCode = if ($result) { 0 } else { 1 }
        }
        "clean" {
            Clear-TestArtifacts
            $exitCode = 0
        }
        "all" {
            Write-Status "Starting comprehensive test suite..."
            
            # Run lint first
            $lintResult = Invoke-Lint
            
            # Run all tests
            $testResult = Run-AllTests
            
            # Run race detection
            $raceResult = Run-RaceTests
            
            # Run coverage
            $coverageResult = Run-CoverageTests
            
            # Determine overall result
            if ($testResult -and $raceResult -and $coverageResult) {
                Write-Success "All test suites passed!"
                $exitCode = 0
            } else {
                Write-Error "Some test suites failed"
                $exitCode = 1
            }
        }
        "help" {
            Show-Usage
            $exitCode = 0
        }
        default {
            Write-Error "Unknown option: $Option"
            Show-Usage
            $exitCode = 1
        }
    }
    
    exit $exitCode
}

# Run main function
Main
