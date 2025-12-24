// PedroCLI Web UI - Main JavaScript

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    console.log('PedroCLI Web UI initialized');

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
});

// Job localStorage management (for Phase 2)
// This will be expanded in Phase 2 to cache jobs with 24hr expiry
const JobCache = {
    // Placeholder for Phase 2 implementation
    save: function(job) {
        console.log('JobCache.save:', job);
        // TODO: Implement in Phase 2
    },

    load: function() {
        console.log('JobCache.load');
        // TODO: Implement in Phase 2
        return [];
    },

    cleanup: function() {
        console.log('JobCache.cleanup');
        // TODO: Implement in Phase 2 - remove jobs older than 24 hours
    }
};
