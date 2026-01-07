// PedroCLI Web UI - Main JavaScript

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    console.log('PedroCLI Web UI initialized');

    // Clean up old cached jobs on startup
    JobCache.cleanup();

    // Connect to real-time job updates
    SSEManager.connectToJobList();

    // Add HTMX trigger for SSE updates
    document.body.addEventListener('sse-update', function(event) {
        console.log('SSE: Triggering HTMX refresh');
        htmx.trigger('#job-list', 'refresh');
    });

    // Reset form after successful job creation
    document.body.addEventListener('htmx:afterSwap', function(event) {
        if (event.detail.target.id === 'job-list' && event.detail.xhr.status === 200) {
            // Reset the form
            const form = document.getElementById('create-job-form');
            if (form) {
                form.reset();
            }
        }
    });

    // Show loading indicator during HTMX requests
    document.body.addEventListener('htmx:beforeRequest', function(event) {
        event.detail.elt.classList.add('opacity-50', 'pointer-events-none');
    });

    document.body.addEventListener('htmx:afterRequest', function(event) {
        event.detail.elt.classList.remove('opacity-50', 'pointer-events-none');
    });

    // Error handling
    document.body.addEventListener('htmx:responseError', function(event) {
        console.error('HTMX Error:', event.detail);
        alert('An error occurred: ' + event.detail.xhr.statusText);
    });

    // Cleanup SSE connections when page unloads
    window.addEventListener('beforeunload', function() {
        SSEManager.disconnectAll();
    });
});

// Job localStorage management with 24hr expiry
const JobCache = {
    CACHE_KEY: 'pedrocli_jobs',
    EXPIRY_HOURS: 24,

    // Save a job to localStorage
    save: function(job) {
        try {
            const jobs = this.load();
            const timestamp = new Date().getTime();

            // Add or update job
            const existingIndex = jobs.findIndex(j => j.id === job.id);
            const jobWithTimestamp = { ...job, _cached_at: timestamp };

            if (existingIndex >= 0) {
                jobs[existingIndex] = jobWithTimestamp;
            } else {
                jobs.push(jobWithTimestamp);
            }

            localStorage.setItem(this.CACHE_KEY, JSON.stringify(jobs));
            console.log('JobCache.save:', job.id);
        } catch (e) {
            console.error('Failed to save job to cache:', e);
        }
    },

    // Load all jobs from localStorage (excluding expired ones)
    load: function() {
        try {
            const data = localStorage.getItem(this.CACHE_KEY);
            if (!data) {
                return [];
            }

            const jobs = JSON.parse(data);
            const now = new Date().getTime();
            const expiryMs = this.EXPIRY_HOURS * 60 * 60 * 1000;

            // Filter out expired jobs
            const validJobs = jobs.filter(job => {
                const age = now - (job._cached_at || 0);
                return age < expiryMs;
            });

            // Update cache if we filtered anything
            if (validJobs.length !== jobs.length) {
                localStorage.setItem(this.CACHE_KEY, JSON.stringify(validJobs));
            }

            console.log('JobCache.load:', validJobs.length, 'jobs');
            return validJobs;
        } catch (e) {
            console.error('Failed to load jobs from cache:', e);
            return [];
        }
    },

    // Remove jobs older than 24 hours
    cleanup: function() {
        try {
            const jobs = this.load(); // load() already filters expired jobs
            console.log('JobCache.cleanup: kept', jobs.length, 'jobs');
        } catch (e) {
            console.error('Failed to cleanup job cache:', e);
        }
    },

    // Clear all cached jobs
    clear: function() {
        try {
            localStorage.removeItem(this.CACHE_KEY);
            console.log('JobCache.clear: all jobs removed');
        } catch (e) {
            console.error('Failed to clear job cache:', e);
        }
    },

    // Get a specific job by ID
    get: function(jobId) {
        const jobs = this.load();
        return jobs.find(j => j.id === jobId);
    }
};

// SSE Connection Manager for real-time job updates
const SSEManager = {
    connections: new Map(),

    // Connect to SSE stream for a specific job
    connect: function(jobId) {
        if (this.connections.has(jobId)) {
            console.log('SSE: Already connected to job', jobId);
            return;
        }

        const url = `/api/stream/jobs/${jobId}`;
        console.log('SSE: Connecting to', url);

        const eventSource = new EventSource(url);

        eventSource.addEventListener('update', (e) => {
            try {
                const data = JSON.parse(e.data);
                console.log('SSE: Job update received:', data);

                // Cache the updated job
                JobCache.save({
                    id: data.data.job_id,
                    status: data.data.status
                });

                // Trigger HTMX refresh of job list (only if element exists)
                const jobList = document.getElementById('job-list');
                if (jobList) {
                    htmx.trigger('#job-list', 'sse-update');
                }
            } catch (err) {
                console.error('SSE: Failed to handle update:', err);
            }
        });

        eventSource.addEventListener('list', (e) => {
            try {
                const data = JSON.parse(e.data);
                console.log('SSE: Job list received');

                // Trigger HTMX refresh (only if element exists)
                const jobList = document.getElementById('job-list');
                if (jobList) {
                    htmx.trigger('#job-list', 'sse-update');
                }
            } catch (err) {
                console.error('SSE: Failed to handle list:', err);
            }
        });

        eventSource.onerror = (err) => {
            console.error('SSE: Connection error for job', jobId, err);
            eventSource.close();
            this.connections.delete(jobId);

            // Retry connection after 5 seconds
            setTimeout(() => {
                console.log('SSE: Retrying connection for job', jobId);
                this.connect(jobId);
            }, 5000);
        };

        this.connections.set(jobId, eventSource);
    },

    // Disconnect from SSE stream for a specific job
    disconnect: function(jobId) {
        const eventSource = this.connections.get(jobId);
        if (eventSource) {
            eventSource.close();
            this.connections.delete(jobId);
            console.log('SSE: Disconnected from job', jobId);
        }
    },

    // Disconnect all SSE streams
    disconnectAll: function() {
        this.connections.forEach((eventSource, jobId) => {
            eventSource.close();
            console.log('SSE: Disconnected from job', jobId);
        });
        this.connections.clear();
    },

    // Connect to the global job list stream
    connectToJobList: function() {
        this.connect('*');
    }
};
