package repl

// Job execution limits
const (
	// MaxConcurrentJobs is the maximum number of jobs that can run simultaneously
	MaxConcurrentJobs = 3

	// MaxUnfinishedJobs is the maximum number of incomplete jobs to keep in storage
	// Older unfinished jobs are deleted when this limit is exceeded
	MaxUnfinishedJobs = 10
)
