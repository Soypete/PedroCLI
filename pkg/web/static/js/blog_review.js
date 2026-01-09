// Blog Review Interface JavaScript

let currentPost = null;
let postId = null;

// Initialize page
document.addEventListener('DOMContentLoaded', function() {
    // Get post ID from URL
    const pathParts = window.location.pathname.split('/');
    postId = pathParts[pathParts.length - 1];

    if (postId && postId !== 'review') {
        loadPost(postId);
        loadVersionHistory(postId);
    }

    // Setup word count
    const content = document.getElementById('post-content');
    if (content) {
        content.addEventListener('input', updateWordCount);
    }

    // Setup social media character counts
    setupCharacterCounters();
});

// Load post from API
async function loadPost(id) {
    try {
        const response = await fetch(`/api/blog/posts/${id}`);
        if (!response.ok) {
            throw new Error('Failed to load post');
        }

        currentPost = await response.json();
        renderPost(currentPost);
    } catch (error) {
        console.error('Error loading post:', error);
        showToast('Error loading post: ' + error.message, 'error');
    }
}

// Render post data
function renderPost(post) {
    document.getElementById('post-title').textContent = post.title || 'Untitled';
    document.getElementById('post-status').textContent = post.status || 'unknown';
    document.getElementById('post-status').className = getStatusClass(post.status);
    document.getElementById('post-created').textContent = 'Created: ' + formatDate(post.created_at);
    document.getElementById('post-version').textContent = 'Version: ' + (post.current_version || 1);

    document.getElementById('post-content').value = post.final_content || '';
    updateWordCount();

    // Social posts
    if (post.social_posts) {
        document.getElementById('social-twitter').value = post.social_posts.twitter || '';
        document.getElementById('social-bluesky').value = post.social_posts.bluesky || '';
        document.getElementById('social-linkedin').value = post.social_posts.linkedin || '';
        updateSocialCharCounts();
    }

    // Editor feedback
    if (post.editor_output) {
        document.getElementById('editor-output').innerHTML = formatMarkdown(post.editor_output);
    }
}

// Load version history
async function loadVersionHistory(id) {
    try {
        const response = await fetch(`/api/blog/posts/${id}`);
        if (!response.ok) {
            throw new Error('Failed to load versions');
        }

        const data = await response.json();
        if (data.versions && data.versions.length > 0) {
            renderVersionHistory(data.versions);
        }
    } catch (error) {
        console.error('Error loading versions:', error);
    }
}

// Render version history list
function renderVersionHistory(versions) {
    const listEl = document.getElementById('version-list');
    if (!versions || versions.length === 0) {
        listEl.innerHTML = '<div class="text-sm text-gray-500 text-center py-4">No versions yet</div>';
        return;
    }

    listEl.innerHTML = versions.map(v => `
        <div class="border border-gray-200 rounded-lg p-3 hover:bg-gray-50 cursor-pointer"
             onclick="loadVersion(${v.version_number})">
            <div class="flex items-center justify-between mb-1">
                <span class="text-sm font-medium">v${v.version_number}</span>
                <span class="text-xs text-gray-500">${v.version_type}</span>
            </div>
            <div class="text-xs text-gray-600">${v.phase || 'N/A'}</div>
            <div class="text-xs text-gray-500 mt-1">${formatDate(v.created_at)}</div>
        </div>
    `).reverse().join('');
}

// Load specific version
async function loadVersion(versionNumber) {
    if (!postId) return;

    try {
        const response = await fetch(`/api/blog/posts/${postId}/versions/${versionNumber}`);
        if (!response.ok) {
            throw new Error('Failed to load version');
        }

        const version = await response.json();
        document.getElementById('post-content').value = version.full_content || '';
        updateWordCount();
        showToast(`Loaded version ${versionNumber}`, 'success');
    } catch (error) {
        console.error('Error loading version:', error);
        showToast('Error loading version: ' + error.message, 'error');
    }
}

// Save manual edit
async function saveManualEdit() {
    if (!postId) return;

    const content = document.getElementById('post-content').value;
    const changeNotes = prompt('Describe your changes (optional):') || '';

    try {
        const response = await fetch(`/api/blog/posts/${postId}/edit`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                content: content,
                change_notes: changeNotes
            })
        });

        if (!response.ok) {
            throw new Error('Failed to save changes');
        }

        const result = await response.json();
        showToast('Changes saved successfully!', 'success');

        // Reload version history
        loadVersionHistory(postId);

        // Update current version number
        if (result.version) {
            document.getElementById('post-version').textContent = 'Version: ' + result.version;
        }
    } catch (error) {
        console.error('Error saving changes:', error);
        showToast('Error saving changes: ' + error.message, 'error');
    }
}

// Request AI revision
async function requestAIRevision() {
    if (!postId) return;

    const prompt = document.getElementById('revision-prompt').value;
    if (!prompt.trim()) {
        showToast('Please describe the changes you want', 'error');
        return;
    }

    // Show progress
    const progressEl = document.getElementById('revision-progress');
    progressEl.classList.remove('hidden');

    try {
        const response = await fetch(`/api/blog/posts/${postId}/revise`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                prompt: prompt
            })
        });

        if (!response.ok) {
            throw new Error('Failed to start revision');
        }

        const result = await response.json();
        if (result.job_id) {
            // Start streaming job updates
            streamJobUpdates(result.job_id);
        }
    } catch (error) {
        console.error('Error requesting revision:', error);
        showToast('Error requesting revision: ' + error.message, 'error');
        progressEl.classList.add('hidden');
    }
}

// Stream job updates via SSE
function streamJobUpdates(jobId) {
    const eventSource = new EventSource(`/api/stream/jobs/${jobId}`);
    const statusEl = document.getElementById('revision-status');

    eventSource.addEventListener('progress', (event) => {
        const data = JSON.parse(event.data);
        statusEl.textContent = data.message || 'Processing...';
    });

    eventSource.addEventListener('complete', (event) => {
        const data = JSON.parse(event.data);
        eventSource.close();

        // Reload the post to get updated content
        loadPost(postId);

        // Hide progress
        document.getElementById('revision-progress').classList.add('hidden');
        document.getElementById('revision-prompt').value = '';

        showToast('Revision complete!', 'success');
    });

    eventSource.addEventListener('error', (event) => {
        console.error('SSE error:', event);
        eventSource.close();
        document.getElementById('revision-progress').classList.add('hidden');
        showToast('Error during revision', 'error');
    });
}

// Save version
async function saveVersion() {
    if (!postId) return;

    const notes = prompt('Version notes (optional):') || '';

    try {
        const response = await fetch(`/api/blog/posts/${postId}/versions`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                notes: notes
            })
        });

        if (!response.ok) {
            throw new Error('Failed to save version');
        }

        showToast('Version saved!', 'success');
        loadVersionHistory(postId);
    } catch (error) {
        console.error('Error saving version:', error);
        showToast('Error saving version: ' + error.message, 'error');
    }
}

// Publish to Notion
async function publishToNotion() {
    if (!postId) return;

    if (!confirm('Publish this post to Notion?')) {
        return;
    }

    try {
        const response = await fetch(`/api/blog/posts/${postId}/publish`, {
            method: 'POST'
        });

        if (!response.ok) {
            throw new Error('Failed to publish');
        }

        const result = await response.json();
        showToast('Published to Notion!', 'success');

        if (result.notion_url) {
            window.open(result.notion_url, '_blank');
        }
    } catch (error) {
        console.error('Error publishing:', error);
        showToast('Error publishing: ' + error.message, 'error');
    }
}

// Voice dictation
function startVoiceDictation() {
    // Check if browser supports SpeechRecognition
    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;

    if (!SpeechRecognition) {
        showToast('Voice recognition not supported in this browser', 'error');
        return;
    }

    const recognition = new SpeechRecognition();
    recognition.continuous = true;
    recognition.interimResults = true;

    const promptEl = document.getElementById('revision-prompt');
    let finalTranscript = promptEl.value;

    recognition.onresult = (event) => {
        let interimTranscript = '';

        for (let i = event.resultIndex; i < event.results.length; i++) {
            const transcript = event.results[i][0].transcript;
            if (event.results[i].isFinal) {
                finalTranscript += transcript + ' ';
            } else {
                interimTranscript += transcript;
            }
        }

        promptEl.value = finalTranscript + interimTranscript;
    };

    recognition.onerror = (event) => {
        console.error('Speech recognition error:', event.error);
        showToast('Voice recognition error: ' + event.error, 'error');
    };

    recognition.start();
    showToast('Listening...', 'success');

    // Stop after 30 seconds
    setTimeout(() => {
        recognition.stop();
    }, 30000);
}

// Utility functions

function updateWordCount() {
    const content = document.getElementById('post-content').value;
    const words = content.trim().split(/\s+/).filter(w => w.length > 0).length;
    document.getElementById('word-count').textContent = `${words} words`;
}

function setupCharacterCounters() {
    const twitter = document.getElementById('social-twitter');
    const bluesky = document.getElementById('social-bluesky');
    const linkedin = document.getElementById('social-linkedin');

    if (twitter) twitter.addEventListener('input', updateSocialCharCounts);
    if (bluesky) bluesky.addEventListener('input', updateSocialCharCounts);
    if (linkedin) linkedin.addEventListener('input', updateSocialCharCounts);
}

function updateSocialCharCounts() {
    const twitter = document.getElementById('social-twitter').value;
    const bluesky = document.getElementById('social-bluesky').value;
    const linkedin = document.getElementById('social-linkedin').value;

    document.getElementById('twitter-chars').textContent = twitter.length;
    document.getElementById('bluesky-chars').textContent = bluesky.length;
    document.getElementById('linkedin-chars').textContent = linkedin.length;
}

function copyToClipboard(elementId) {
    const element = document.getElementById(elementId);
    if (!element) return;

    element.select();
    document.execCommand('copy');
    showToast('Copied to clipboard!', 'success');
}

function downloadMarkdown() {
    const content = document.getElementById('post-content').value;
    const title = document.getElementById('post-title').textContent;
    const filename = title.toLowerCase().replace(/\s+/g, '-') + '.md';

    const blob = new Blob([content], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);

    showToast('Downloaded!', 'success');
}

function viewDiff() {
    // TODO: Implement diff view
    document.getElementById('diff-modal').classList.remove('hidden');
    document.getElementById('diff-content').innerHTML = '<p class="text-gray-500">Diff view coming soon...</p>';
}

function closeDiffModal() {
    document.getElementById('diff-modal').classList.add('hidden');
}

function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    const messageEl = document.getElementById('toast-message');

    messageEl.textContent = message;

    // Set color based on type
    toast.className = type === 'error'
        ? 'fixed bottom-4 right-4 bg-red-600 text-white px-6 py-3 rounded-lg shadow-lg'
        : 'fixed bottom-4 right-4 bg-green-600 text-white px-6 py-3 rounded-lg shadow-lg';

    toast.classList.remove('hidden');

    setTimeout(() => {
        toast.classList.add('hidden');
    }, 3000);
}

function getStatusClass(status) {
    const classes = {
        'dictated': 'px-3 py-1 rounded-full bg-gray-100 text-gray-700',
        'researched': 'px-3 py-1 rounded-full bg-blue-100 text-blue-700',
        'outlined': 'px-3 py-1 rounded-full bg-indigo-100 text-indigo-700',
        'drafted': 'px-3 py-1 rounded-full bg-purple-100 text-purple-700',
        'edited': 'px-3 py-1 rounded-full bg-yellow-100 text-yellow-700',
        'published': 'px-3 py-1 rounded-full bg-green-100 text-green-700'
    };
    return classes[status] || 'px-3 py-1 rounded-full bg-gray-100 text-gray-700';
}

function formatDate(dateString) {
    if (!dateString) return 'N/A';
    const date = new Date(dateString);
    return date.toLocaleString();
}

function formatMarkdown(text) {
    if (!text) return '';

    // Simple markdown formatting
    return text
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        .replace(/\n/g, '<br>');
}
