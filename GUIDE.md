# gosearch - The Complete Beginner's Guide

Welcome! This guide will explain everything about gosearch from the ground up. Even if you've never programmed before, you'll understand how this works by the end.

---

## Part 1: What Problem Does This Solve?

### The Real-World Problem

Imagine you're a detective 🔍 and you need to find every document in a massive filing cabinet that mentions the word "suspect". You have two choices:

1. **The slow way**: Open every single folder, read every single page, and write down where you found the word. This could take days!

2. **The smart way**: Have a team of helpers who each take a section of the cabinet and search simultaneously. They report back to one person who writes down all the findings.

**gosearch is the smart way, but for computer files.**

### Why Do Programmers Need This?

When programmers work on real projects, they deal with:
- **Hundreds or thousands of files** - A typical project might have 500+ files
- **Millions of lines of code** - Large projects have millions of lines
- **Constant searching** - "Where did I define this function?" "Where is this error coming from?"

Without a tool like gosearch, finding something would be like finding a needle in a haystack... blindfolded... in the dark.

### What Makes gosearch Different from Regular Search?

Your computer has a basic search (like Windows Search or Ctrl+F). Here's why gosearch is better for code:

| Regular Search | gosearch |
|----------------|----------|
| Searches one file at a time | Searches ALL files at once |
| Doesn't understand code folders | Knows to skip junk folders like `.git` |
| Just shows "found" or "not found" | Shows exact file, line number, and context |
| Can be slow on big projects | Uses multiple "workers" for speed |
| No pattern matching | Can search for complex patterns (regex) |

---

## Part 2: Understanding the Core Concepts

Before we dive into how gosearch works, let's understand some fundamental concepts.

### Concept 1: What is a File System?

Think of your computer's file system like a tree:

```
Your Computer (Root)
├── Documents/
│   ├── school/
│   │   ├── homework.txt
│   │   └── notes.txt
│   └── personal/
│       └── diary.txt
├── Downloads/
│   └── movie.mp4
└── Code/
    └── my-project/
        ├── main.go
        ├── utils.go
        └── tests/
            └── main_test.go
```

- **Folders** (also called directories) are like branches
- **Files** are like leaves at the end of branches
- **Path** is the address to find something, like `Documents/school/homework.txt`

### Concept 2: What is "Searching" in Files?

When we "search" a file, we're doing this:

1. **Open** the file (like opening a book)
2. **Read** each line one by one
3. **Check** if the line contains our search word
4. **Report** if we found it, including WHERE we found it

For example, searching for "hello" in a file:

```
Line 1: "Welcome to my program"     ← No match
Line 2: "This says hello world"     ← MATCH! (contains "hello")
Line 3: "Goodbye for now"           ← No match
Line 4: "hello again everyone"      ← MATCH! (contains "hello")
```

Result: Found "hello" on lines 2 and 4.

### Concept 3: What is Concurrency? (The Secret Sauce 🚀)

This is the magic that makes gosearch FAST.

**Without Concurrency (Sequential - One at a time):**
```
Worker: Search file1 → done → Search file2 → done → Search file3 → done
Total time: 3 seconds (1 second each)
```

**With Concurrency (Parallel - Multiple at once):**
```
Worker 1: Search file1 → done
Worker 2: Search file2 → done     (all happening at the SAME time!)
Worker 3: Search file3 → done
Total time: 1 second
```

It's like the difference between:
- **One cashier** serving a line of 10 people (slow)
- **Ten cashiers** each serving one person (fast!)

Your computer has multiple "cores" (like having multiple brains), and concurrency uses ALL of them.

### Concept 4: What are Goroutines?

In Go (the programming language gosearch is written in), we use something called **goroutines**.

Think of a goroutine as a **lightweight worker**. You can create thousands of them, and they all work together.

```go
// This creates a new worker that runs independently
go doSomeWork()
```

The `go` keyword is like saying "Hey, start doing this task in the background while I continue with other things."

### Concept 5: What are Channels?

If goroutines are workers, **channels** are the conveyor belts between them.

```
[Worker 1] --puts items on--> [Conveyor Belt] --takes items from--> [Worker 2]
```

In Go:
```go
// Create a conveyor belt that carries "jobs"
jobs := make(chan Job)

// Worker 1 puts a job on the belt
jobs <- newJob

// Worker 2 takes a job from the belt
job := <-jobs
```

This lets workers communicate safely without stepping on each other's toes.

---

## Part 3: How gosearch Works (The Pipeline)

gosearch uses a **pipeline** architecture. Think of it like a factory assembly line:

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  TRAVERSAL  │───▶│ IO WORKERS  │───▶│ CPU WORKERS │───▶│   PRINTER   │
│  (Scout)    │    │ (Readers)   │    │ (Matchers)  │    │ (Reporter)  │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

Let's break down each stage:

### Stage 1: Traversal (The Scout)

**Job**: Walk through all folders and find files to search.

**Real-world analogy**: A scout who walks through a library and makes a list of all books that need to be checked.

**What it does**:
1. Starts at the folder you specify (e.g., `./my-project`)
2. Walks into each subfolder
3. For each file found, asks: "Should I search this?"
4. If yes, puts the file path on a conveyor belt for the next stage

**Smart behaviors**:
- **Ignores certain folders**: Skips `.git`, `node_modules`, `vendor` (these contain thousands of files you don't care about)
- **Respects `.gitignore`**: If a file is in `.gitignore`, it won't search it
- **Handles depth limits**: Can stop at a certain folder depth
- **Prevents infinite loops**: If there are circular folder links (symlinks), it won't get stuck

```
Traversal walks through:
my-project/
├── main.go           ✅ Add to queue
├── utils.go          ✅ Add to queue
├── .git/             ❌ SKIP (ignored folder)
│   └── (hundreds of files)
├── node_modules/     ❌ SKIP (ignored folder)
│   └── (thousands of files)
└── README.md         ✅ Add to queue
```

### Stage 2: IO Workers (The Readers)

**Job**: Open files and read their contents.

**Real-world analogy**: Librarians who take books from the scout's list, open them, and prepare the pages for reading.

**What they do**:
1. Take a file path from the conveyor belt
2. Open the file
3. Read it line by line
4. For each line, put it on another conveyor belt for matching

**Why separate from matching?**
- **IO** (Input/Output) means reading from disk - this is SLOW (like waiting for a slow printer)
- **CPU** work (matching) is FAST (your processor is lightning quick)
- By separating them, slow disk reads don't block fast matching

**Binary file detection**:
```
File: image.png
Content: [weird binary data: 0x89 0x50 0x4E 0x47...]
Decision: SKIP! This is not text, don't waste time searching it.
```

### Stage 3: CPU Workers (The Matchers)

**Job**: Check if each line contains the search pattern.

**Real-world analogy**: Speed readers who quickly scan each page and mark any that contain your search word.

**What they do**:
1. Take a line from the conveyor belt
2. Apply the matching strategy (more on this below)
3. If it matches, put the result on the results conveyor belt

**Matching Strategies**:

**Strategy A: Substring Matching (Default)**
```
Search for: "hello"
Line: "Say hello to the world"
Method: Does the line contain "hello"? → YES, MATCH!
```

**Strategy B: Regex Matching (Pattern-based)**
```
Search for: "func\s+\w+"  (means: "func" followed by spaces and a word)
Line: "func    main() {"
Method: Does the line match this pattern? → YES, MATCH!
```

**Extra options**:
- **Case-insensitive** (`-i`): "HELLO" matches "hello"
- **Whole word** (`-w`): "hello" won't match "helloworld"

### Stage 4: Printer (The Reporter)

**Job**: Display results to the user.

**Real-world analogy**: A single secretary who collects all findings and writes the final report.

**Why only ONE printer?**
This is crucial! If multiple workers tried to print at the same time:

```
Bad (multiple printers):
file1.go:10: Worker1 says hWorker2 says ello wofile3.go:5: rld
                     ^^^^^^^^^^^^^^^^^^^^^^
                     GARBLED MESS!

Good (single printer):
file1.go:10: hello world
file2.go:25: hello there
file3.go:5: say hello
```

The single printer ensures clean, ungarbled output.

**Output formats**:

**Plain text** (default):
```
main.go:15: fmt.Println("hello world")
utils.go:42: // hello this is a comment
```

**JSON** (for other programs to read):
```json
{"path":"main.go","line":15,"text":"fmt.Println(\"hello world\")"}
{"path":"utils.go","line":42,"text":"// hello this is a comment"}
```

---

## Part 4: The Data Structures

### What is a Struct?

In Go, a **struct** is a way to group related data together. Think of it like a form with multiple fields:

```go
// A "Result" struct - represents one search match
type Result struct {
    Path   string      // Which file? e.g., "main.go"
    Line   int         // Which line number? e.g., 15
    Text   string      // What did the line say? e.g., "hello world"
    Ranges []MatchRange // Where exactly in the line?
}
```

### Key Data Structures in gosearch

**1. Config** - Stores all settings
```go
type Config struct {
    pattern      string  // What to search for: "hello"
    rootPath     string  // Where to search: "./my-project"
    ignoreCase   bool    // Ignore uppercase/lowercase? true/false
    workers      int     // How many workers? 4
    // ... many more settings
}
```

**2. Result** - Represents one match found
```go
type Result struct {
    Path   string       // "src/main.go"
    Line   int          // 42
    Text   string       // "fmt.Println(\"hello\")"
    Ranges []MatchRange // [{Start: 14, End: 19}] - where "hello" is
}
```

**3. MatchRange** - Exactly where in the line the match is
```go
type MatchRange struct {
    Start int  // Character position where match starts
    End   int  // Character position where match ends
}

// Example: "Say hello world"
//           01234567890123456
// "hello" is at positions 4-9
// MatchRange{Start: 4, End: 9}
```

---

## Part 5: The Ignore System

### Why Ignore Files?

When you search a code project, you DON'T want to search:
- `.git/` folder - Contains Git's internal files (thousands of them!)
- `node_modules/` - JavaScript dependencies (can have 50,000+ files!)
- `vendor/` - Go dependencies
- Binary files - Images, executables, etc.

Searching these would be:
1. **Slow** - Tons of unnecessary files
2. **Noisy** - Results you don't care about
3. **Wasteful** - Your computer working for nothing

### How .gitignore Works

`.gitignore` is a file that tells Git (and gosearch) what to ignore:

```gitignore
# This is a comment

# Ignore all .log files
*.log

# Ignore the build folder
build/

# Ignore node_modules anywhere
node_modules/

# But DON'T ignore this specific log file (! means "not")
!important.log
```

gosearch reads these rules and skips matching files/folders.

### How .gosearchignore Works

Same as `.gitignore`, but specific to gosearch. This lets you ignore things for searching that you might still want in Git.

### Inheritance

Ignore rules **inherit** down the folder tree:

```
project/
├── .gitignore          (says: ignore *.log)
├── logs/
│   └── debug.log       ← IGNORED (parent says ignore *.log)
└── src/
    ├── .gitignore      (says: !important.log - override!)
    ├── important.log   ← NOT IGNORED (local rule overrides)
    └── other.log       ← IGNORED (parent rule still applies)
```

---

## Part 6: Symlinks and Loop Prevention

### What is a Symlink?

A **symbolic link** (symlink) is like a shortcut. It points to another file or folder.

```
project/
├── real-folder/
│   └── data.txt
└── shortcut -> real-folder/   (this is a symlink)
```

If you access `shortcut/data.txt`, you're actually accessing `real-folder/data.txt`.

### The Danger: Infinite Loops

What if symlinks create a circle?

```
folder-a/
├── link-to-b -> ../folder-b/
└── file1.txt

folder-b/
├── link-to-a -> ../folder-a/   ← DANGER!
└── file2.txt
```

Without protection:
```
Enter folder-a
  → Follow link-to-b → Enter folder-b
    → Follow link-to-a → Enter folder-a
      → Follow link-to-b → Enter folder-b
        → (FOREVER... until your computer crashes)
```

### How gosearch Prevents This

gosearch keeps track of folders it has already visited:

```go
visitedDirs := map[string]bool{
    "/home/user/folder-a": true,  // Already visited!
}

// When we try to enter folder-a again via symlink:
if visitedDirs[resolvedPath] {
    // "I've been here before! SKIP to prevent loop."
    return
}
```

By default, gosearch **skips symlinks entirely**. You can enable following them with `-follow-symlinks`, and gosearch will handle loops safely.

---

## Part 7: Context and Cancellation

### What is Context?

**Context** is Go's way of saying "Here's some background information, and here's how to cancel this operation."

Think of it like a "stop button" that works across all workers:

```
[Main Program]
      |
      ├── "Start searching!"
      |
      ├──[Worker 1]──┐
      ├──[Worker 2]──┼── All workers share the same "stop button"
      └──[Worker 3]──┘
      
User presses Ctrl+C:
      |
      └── "STOP!" signal sent to ALL workers simultaneously
```

### Why This Matters

Without proper cancellation:
```
User: "Search for 'foo' in this huge project"
Program: *starts searching 10,000 files*
User: "Wait, I made a typo! Ctrl+C!"
Bad Program: *ignores you and keeps searching*
User: *frustrated*
```

With proper cancellation (gosearch):
```
User: "Search for 'foo' in this huge project"  
Program: *starts searching 10,000 files*
User: "Wait, I made a typo! Ctrl+C!"
gosearch: *immediately stops all workers cleanly*
User: *happy*
```

### How It Works in Code

```go
// Create a context that cancels when user presses Ctrl+C
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

// All workers check this context
func worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            // Someone said stop! Clean up and exit.
            return
        case job := <-jobs:
            // Do work...
        }
    }
}
```

---

## Part 8: Exit Codes

### What are Exit Codes?

When a program finishes, it returns a number to tell the system how it went:

- **0** = "Everything went great!"
- **Non-zero** = "Something noteworthy happened"

This is important for **scripting** - when one program's output affects what another program does.

### gosearch's Exit Codes

| Code | Meaning | Example |
|------|---------|---------|
| **0** | Found matches | `gosearch "TODO" .` → Found 5 TODOs |
| **1** | No matches found | `gosearch "xyzzy123" .` → Nothing matched |
| **2** | Error occurred | `gosearch "" .` → Empty pattern is invalid |

### Why This Matters for Scripts

```bash
#!/bin/bash

# Search for security issues
gosearch "password" ./src

# Check the exit code
if [ $? -eq 0 ]; then
    echo "WARNING: Found hardcoded passwords!"
    exit 1  # Fail the build
else
    echo "OK: No passwords found in code"
fi
```

This is how automated systems (like CI/CD) can make decisions based on search results.

---

## Part 9: Backpressure and Flow Control

### The Problem: Too Fast!

What if one stage is faster than another?

```
[Super Fast Traversal] → [Slow IO Workers]

Traversal: "Here's 10,000 files to search!"
IO Workers: "Whoa, I can only handle 10 at a time!"
```

Without control, memory fills up with waiting jobs until your computer crashes.

### The Solution: Backpressure

**Backpressure** means "slow down the fast parts to match the slow parts."

In gosearch, channels have a **buffer size**:

```go
// Create a channel that can hold 100 items max
pathJobs := make(chan PathJob, 100)
```

When the buffer is full:
- Fast producer: "I'll wait until there's room" (blocks)
- Slow consumer: *catches up*
- Fast producer: "Room opened up! Continuing..."

It's like a highway on-ramp with a traffic light - it prevents gridlock.

### Configuring Backpressure

```bash
# Set channel buffer size to 500
gosearch -backpressure 500 "pattern" .
```

- **Higher** = More memory usage, less waiting
- **Lower** = Less memory, more coordination overhead

---

## Part 10: Dynamic Worker Scaling

### The Problem: Different Workloads

Some searches are:
- **IO-heavy**: Many small files (bottleneck: disk reads)
- **CPU-heavy**: Few large files with complex regex (bottleneck: matching)

Fixed worker counts can't adapt.

### The Solution: Dynamic Scaling

gosearch can **add more CPU workers** when they're overwhelmed:

```bash
gosearch -dynamic-workers -max-workers 16 "pattern" .
```

How it works:
1. Start with base number of CPU workers
2. Monitor the line jobs queue
3. If queue is backing up → spawn more workers
4. Cap at max-workers to prevent runaway scaling

```
Initial state:
[IO Workers] → [Queue: 5 items] → [4 CPU Workers]

Queue backing up:
[IO Workers] → [Queue: 50 items] → [4 CPU Workers] "Overwhelmed!"

Dynamic scaling kicks in:
[IO Workers] → [Queue: 50 items] → [8 CPU Workers] "Catching up!"

Queue drains:
[IO Workers] → [Queue: 3 items] → [8 CPU Workers] "Stable!"
```

---

## Part 11: Command Line Flags Reference

### What are Flags?

Flags are options you pass to a command to change its behavior:

```bash
gosearch -i -w "pattern" ./path
#        ^^  ^^
#        Flags!
```

### All gosearch Flags Explained

#### Search Behavior

| Flag | What it does | Example |
|------|--------------|---------|
| `-i` | Ignore case (A = a) | `gosearch -i "hello"` matches "HELLO" |
| `-w` | Whole word only | `gosearch -w "the"` won't match "there" |
| `-regex` | Use pattern matching | `gosearch -regex "func\s+\w+"` |
| `-n` | Show line numbers | On by default, `-n=false` to hide |

#### Filtering

| Flag | What it does | Example |
|------|--------------|---------|
| `-extensions` | Only these file types | `-extensions .go,.js` |
| `-exclude-dir` | Skip these folders | `-exclude-dir vendor,dist` |
| `-max-size` | Skip large files | `-max-size 1MB` |
| `-max-depth` | Limit folder depth | `-max-depth 3` |
| `-follow-symlinks` | Follow shortcuts | Off by default (safer) |

#### Output

| Flag | What it does | Example |
|------|--------------|---------|
| `-format` | Output style | `-format json` for machine-readable |
| `-count` | Just show count | "Found 42 matches" |
| `-quiet` | No output, just exit code | For scripts |
| `-color` | Highlight matches | Pretty terminal output |
| `-abs` | Show full file paths | `/home/user/...` vs `./...` |

#### Performance

| Flag | What it does | Example |
|------|--------------|---------|
| `-workers` | Base worker count | `-workers 8` |
| `-io-workers` | File reading workers | `-io-workers 4` |
| `-cpu-workers` | Pattern matching workers | `-cpu-workers 8` |
| `-dynamic-workers` | Auto-scale CPU workers | Enable with flag |
| `-max-workers` | Cap for dynamic scaling | `-max-workers 16` |
| `-backpressure` | Queue buffer size | `-backpressure 200` |

#### Debugging

| Flag | What it does | Example |
|------|--------------|---------|
| `-metrics` | Show performance stats | Throughput, timings |
| `-debug` | Debug logging | More output |
| `-trace` | Verbose tracing | Even more output |
| `-cpuprofile` | Save CPU profile | `-cpuprofile cpu.out` |
| `-memprofile` | Save memory profile | `-memprofile mem.out` |

---

## Part 12: Configuration File

### Why a Config File?

If you always use the same flags, typing them every time is tedious:

```bash
# Every. Single. Time.
gosearch -i -w -format json -workers 8 -extensions .go "pattern" .
```

### The .gosearchrc File

Create a `.gosearchrc` file with your defaults:

```json
{
    "ignore_case": true,
    "whole_word": true,
    "format": "json",
    "workers": 8,
    "extensions": [".go"]
}
```

Now just run:
```bash
gosearch "pattern" .
# Uses all the settings from .gosearchrc!
```

### Precedence (What Wins?)

```
CLI Flags > Config File > Built-in Defaults

gosearch -i=false "pattern" .
         ^^^^^^^^
         This wins, even if config says ignore_case: true
```

---

## Part 13: Understanding the Code Flow

Let's trace what happens when you run:

```bash
gosearch "TODO" ./my-project
```

### Step 1: Parse Command Line

```go
// main() starts
func main() {
    cfg := parseFlags()  // Read -i, -w, etc.
    // cfg.pattern = "TODO"
    // cfg.rootPath = "./my-project"
}
```

### Step 2: Set Up Signal Handling

```go
// If user presses Ctrl+C, cancel everything
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()
```

### Step 3: Create the Pipeline

```go
// Create conveyor belts (channels)
pathJobs := make(chan PathJob, bufferSize)
lineJobs := make(chan LineJob, bufferSize)
results := make(chan Result, bufferSize)

// Start workers
go walkFiles(ctx, cfg, pathJobs)           // Stage 1: Find files
go startIOWorkers(ctx, pathJobs, lineJobs) // Stage 2: Read files
go startCPUWorkers(ctx, lineJobs, results) // Stage 3: Match patterns
summary := printResults(ctx, results)       // Stage 4: Print matches
```

### Step 4: Wait for Completion

```go
// All workers finish when their input channels close
// printResults returns when results channel closes
return exitCode(summary)
```

### Visual Flow

```
User runs: gosearch "TODO" ./my-project
                    ↓
            ┌──────────────────┐
            │   Parse Flags    │
            │ pattern="TODO"   │
            │ path="./project" │
            └────────┬─────────┘
                     ↓
            ┌──────────────────┐
            │  Setup Context   │
            │  (Ctrl+C ready)  │
            └────────┬─────────┘
                     ↓
            ┌──────────────────┐
            │ Start Traversal  │ → Finds: main.go, util.go, test.go
            └────────┬─────────┘
                     ↓
            ┌──────────────────┐
            │ IO Workers Read  │ → Opens each file, reads lines
            └────────┬─────────┘
                     ↓
            ┌──────────────────┐
            │ CPU Workers      │ → Checks: does line contain "TODO"?
            │ Match Lines      │
            └────────┬─────────┘
                     ↓
            ┌──────────────────┐
            │ Printer Outputs  │ → main.go:15: // TODO: fix this
            └────────┬─────────┘
                     ↓
            ┌──────────────────┐
            │ Return Exit Code │ → 0 (matches found!)
            └──────────────────┘
```

---

## Part 14: Testing (How We Know It Works)

### Why Test?

Without tests, we'd have to manually check every feature after every change. That's:
- Slow
- Error-prone
- Boring

Automated tests run in seconds and catch bugs automatically.

### Types of Tests in gosearch

**1. Unit Tests** - Test small pieces in isolation
```go
func TestMatcherFindsSubstring(t *testing.T) {
    matcher := NewMatcher("hello", false, false)
    result := matcher.Match("say hello world")
    if !result {
        t.Error("Expected to find 'hello'")
    }
}
```

**2. Integration Tests** - Test the whole system
```go
func TestFullSearchFindsMatches(t *testing.T) {
    // Run gosearch on test files
    // Check that output contains expected matches
}
```

**3. Benchmark Tests** - Measure performance
```go
func BenchmarkSearch(b *testing.B) {
    for i := 0; i < b.N; i++ {
        // Run search and measure time
    }
}
// Output: BenchmarkSearch    1000    1234567 ns/op
```

**4. Fuzz Tests** - Try random inputs to find crashes
```go
func FuzzSearch(f *testing.F) {
    f.Fuzz(func(t *testing.T, pattern string) {
        // Try random patterns
        // Make sure gosearch doesn't crash
    })
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with race detector (finds concurrency bugs)
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# Run fuzz tests
go test -fuzz=Fuzz ./...
```

---

## Part 15: Glossary

| Term | Definition |
|------|------------|
| **Binary file** | A file with non-text data (images, executables) |
| **Buffer** | Temporary storage space |
| **Channel** | A pipe for sending data between goroutines |
| **CLI** | Command Line Interface (text-based program) |
| **Concurrency** | Running multiple tasks at overlapping times |
| **Context** | Go's mechanism for cancellation and deadlines |
| **Exit code** | Number returned when a program ends (0 = success) |
| **Flag** | A command-line option like `-i` or `--help` |
| **Goroutine** | A lightweight thread in Go |
| **IO** | Input/Output (reading/writing files, network, etc.) |
| **Pipeline** | Stages of processing connected in sequence |
| **Regex** | Regular Expression (pattern matching language) |
| **Struct** | A Go data type grouping related fields |
| **Symlink** | A file that points to another file/folder |
| **Traversal** | Walking through a directory tree |

---

## Quick Start Examples

```bash
# Basic search
gosearch "error" ./src

# Case-insensitive, whole word
gosearch -i -w "config" .

# Only Go files, show line numbers
gosearch -extensions .go -n "func" .

# JSON output for scripts
gosearch -format json "TODO" . > todos.json

# Quick check (just exit code)
gosearch -quiet "FIXME" . && echo "Found FIXMEs!"

# Follow symlinks safely
gosearch -follow-symlinks "import" ./project

# Performance tuning
gosearch -workers 8 -dynamic-workers "pattern" ./huge-project

# Debugging
gosearch -metrics -debug "needle" ./haystack
```

---

## Summary

gosearch is a **concurrent file search tool** that:

1. **Walks** through your folders intelligently (skipping junk)
2. **Reads** files in parallel using IO workers
3. **Matches** patterns using CPU workers
4. **Prints** results through a single coordinated output

It's fast because it uses **goroutines** (lightweight threads) and **channels** (safe communication pipes).

It's safe because it handles **cancellation** cleanly and prevents **infinite loops**.

It's practical because it understands **.gitignore**, supports **regex**, and has **flexible output formats**.

Welcome to gosearch! 🔍
