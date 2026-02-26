# Architecture Diagram

```mermaid
flowchart TD
    CLI[CLI Flags + Config] --> Traverse[Traversal Engine]
    Traverse --> FileJobs[(pathJobs)]
    FileJobs --> IOWorkers[IO Workers]
    IOWorkers --> LineJobs[(lineJobs)]
    LineJobs --> CPUWorkers[CPU Workers + MatchStrategy]
    CPUWorkers --> Results[(results)]
    Results --> Printer[Single Printer]
    Printer --> Output[stdout]
    Printer --> Exit[Exit Code]
```

## Notes

- Traversal handles ignore rules, depth limits, symlink policy, and enqueue pruning.
- IO workers handle file access, binary checks, and line extraction.
- CPU workers handle substring/regex matching through a strategy interface.
- Printer is the only output writer and controls final result counting.
