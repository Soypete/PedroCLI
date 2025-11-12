// PedroCLI Web UI JavaScript
// Handles WebSocket communication, agent interaction, and voice recording

class PedroCLIApp {
    constructor() {
        this.ws = null;
        this.selectedAgent = null;
        this.inputMode = 'text';
        this.currentJobId = null;
        this.mediaRecorder = null;
        this.audioChunks = [];
        this.jobs = [];

        this.init();
    }

    init() {
        this.connectWebSocket();
        this.setupEventListeners();
        this.loadRecentJobs();
    }

    // ========== WebSocket Connection ==========

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.showStatus('Connected to server', 'success');
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleWebSocketMessage(message);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.showStatus('Connection error', 'error');
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.showStatus('Disconnected from server', 'warning');

            // Attempt reconnect after 3 seconds
            setTimeout(() => this.connectWebSocket(), 3000);
        };
    }

    handleWebSocketMessage(message) {
        switch (message.type) {
            case 'job_created':
                this.handleJobCreated(message.job);
                break;
            case 'job_update':
                this.handleJobUpdate(message.job);
                break;
            case 'job_completed':
                this.handleJobCompleted(message.job);
                break;
            case 'job_failed':
                this.handleJobFailed(message.job, message.error);
                break;
            case 'agents_list':
                this.handleAgentsList(message.agents);
                break;
            case 'jobs_list':
                this.handleJobsList(message.jobs);
                break;
            default:
                console.log('Unknown message type:', message.type);
        }
    }

    sendWebSocketMessage(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            this.showStatus('Not connected to server', 'error');
        }
    }

    // ========== Agent Selection ==========

    setupEventListeners() {
        // Agent card selection
        document.querySelectorAll('.agent-card').forEach(card => {
            card.addEventListener('click', () => {
                const agentType = card.dataset.agent;
                this.selectAgent(agentType);
            });
        });

        // Input mode toggle
        document.querySelectorAll('.mode-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const mode = btn.dataset.mode;
                this.switchInputMode(mode);
            });
        });

        // Form submissions
        document.getElementById('build-form')?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitBuildJob();
        });

        document.getElementById('debug-form')?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitDebugJob();
        });

        document.getElementById('review-form')?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitReviewJob();
        });

        document.getElementById('triage-form')?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.submitTriageJob();
        });

        // Voice recording
        document.getElementById('record-btn')?.addEventListener('click', () => {
            this.toggleRecording();
        });
    }

    selectAgent(agentType) {
        this.selectedAgent = agentType;

        // Update UI
        document.querySelectorAll('.agent-card').forEach(card => {
            card.classList.remove('selected');
        });
        document.querySelector(`[data-agent="${agentType}"]`)?.classList.add('selected');

        // Show appropriate form
        document.querySelectorAll('.agent-form').forEach(form => {
            form.style.display = 'none';
        });
        document.getElementById(`${agentType}-form`)?.parentElement?.parentElement?.setAttribute('style', 'display: block');

        // Update voice transcript placeholder
        this.updateVoicePlaceholder();
    }

    switchInputMode(mode) {
        this.inputMode = mode;

        // Update UI
        document.querySelectorAll('.mode-btn').forEach(btn => {
            btn.classList.remove('active');
        });
        document.querySelector(`[data-mode="${mode}"]`)?.classList.add('active');

        // Show/hide appropriate input areas
        if (mode === 'text') {
            document.querySelectorAll('.text-input').forEach(el => el.style.display = 'block');
            document.querySelector('.voice-area').style.display = 'none';
        } else {
            document.querySelectorAll('.text-input').forEach(el => el.style.display = 'none');
            document.querySelector('.voice-area').style.display = 'block';
        }
    }

    // ========== Job Submission ==========

    submitBuildJob() {
        const description = document.getElementById('build-description').value;
        const issue = document.getElementById('build-issue').value;

        if (!description) {
            this.showStatus('Please provide a feature description', 'warning');
            return;
        }

        this.sendWebSocketMessage({
            type: 'run_agent',
            agent: 'builder',
            input: {
                description: description,
                issue_reference: issue || ''
            }
        });

        this.clearForm('build-form');
    }

    submitDebugJob() {
        const symptoms = document.getElementById('debug-symptoms').value;
        const logs = document.getElementById('debug-logs').value;

        if (!symptoms) {
            this.showStatus('Please describe the symptoms', 'warning');
            return;
        }

        this.sendWebSocketMessage({
            type: 'run_agent',
            agent: 'debugger',
            input: {
                symptoms: symptoms,
                log_files: logs || ''
            }
        });

        this.clearForm('debug-form');
    }

    submitReviewJob() {
        const branch = document.getElementById('review-branch').value;
        const pr = document.getElementById('review-pr').value;

        if (!branch) {
            this.showStatus('Please provide a branch name', 'warning');
            return;
        }

        this.sendWebSocketMessage({
            type: 'run_agent',
            agent: 'reviewer',
            input: {
                branch: branch,
                pr_number: pr || ''
            }
        });

        this.clearForm('review-form');
    }

    submitTriageJob() {
        const description = document.getElementById('triage-description').value;

        if (!description) {
            this.showStatus('Please provide an issue description', 'warning');
            return;
        }

        this.sendWebSocketMessage({
            type: 'run_agent',
            agent: 'triager',
            input: {
                description: description
            }
        });

        this.clearForm('triage-form');
    }

    clearForm(formId) {
        document.getElementById(formId)?.reset();
    }

    // ========== Voice Recording ==========

    async toggleRecording() {
        const recordBtn = document.getElementById('record-btn');
        const statusEl = document.querySelector('.recording-status');

        if (!this.mediaRecorder || this.mediaRecorder.state === 'inactive') {
            // Start recording
            try {
                const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
                this.mediaRecorder = new MediaRecorder(stream);
                this.audioChunks = [];

                this.mediaRecorder.ondataavailable = (event) => {
                    this.audioChunks.push(event.data);
                };

                this.mediaRecorder.onstop = () => {
                    this.processRecording();
                };

                this.mediaRecorder.start();
                recordBtn.classList.add('recording');
                recordBtn.textContent = '⏹️ Stop Recording';
                statusEl.textContent = 'Recording... Click to stop';
            } catch (error) {
                console.error('Error accessing microphone:', error);
                this.showStatus('Microphone access denied', 'error');
            }
        } else {
            // Stop recording
            this.mediaRecorder.stop();
            this.mediaRecorder.stream.getTracks().forEach(track => track.stop());
            recordBtn.classList.remove('recording');
            recordBtn.textContent = '🎤 Start Recording';
            statusEl.textContent = 'Processing...';
        }
    }

    async processRecording() {
        const audioBlob = new Blob(this.audioChunks, { type: 'audio/webm' });

        // Send to whisper.cpp endpoint for transcription
        const formData = new FormData();
        formData.append('audio', audioBlob);

        try {
            const response = await fetch('/api/transcribe', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                throw new Error('Transcription failed');
            }

            const data = await response.json();
            this.handleTranscription(data.text);
        } catch (error) {
            console.error('Transcription error:', error);
            this.showStatus('Failed to transcribe audio. Make sure whisper.cpp is running.', 'error');
            document.querySelector('.recording-status').textContent = 'Ready to record';
        }
    }

    handleTranscription(text) {
        const transcriptEl = document.querySelector('.voice-transcript p');
        transcriptEl.textContent = text;

        document.querySelector('.recording-status').textContent = 'Ready to record';

        // Auto-submit based on selected agent
        if (this.selectedAgent) {
            this.submitVoiceInput(text);
        } else {
            this.showStatus('Please select an agent first', 'warning');
        }
    }

    submitVoiceInput(text) {
        // Parse the text to extract relevant information
        // For now, use the entire text as description/symptoms
        const input = { description: text };

        this.sendWebSocketMessage({
            type: 'run_agent',
            agent: this.selectedAgent,
            input: input
        });
    }

    updateVoicePlaceholder() {
        const transcriptEl = document.querySelector('.voice-transcript p');
        if (!this.selectedAgent) {
            transcriptEl.textContent = 'Select an agent above, then record your request...';
        } else {
            const placeholders = {
                builder: 'Describe the feature you want to build...',
                debugger: 'Describe the bug or issue you\'re experiencing...',
                reviewer: 'Say the branch name to review...',
                triager: 'Describe the issue you want to triage...'
            };
            transcriptEl.textContent = placeholders[this.selectedAgent] || 'Record your request...';
        }
    }

    // ========== Job Management ==========

    handleJobCreated(job) {
        this.currentJobId = job.id;
        this.showOutputSection();
        this.updateJobStatus(job);
        this.showStatus(`Job ${job.id} started`, 'success');
    }

    handleJobUpdate(job) {
        this.updateJobStatus(job);
        this.appendLog(job.progress || '');
    }

    handleJobCompleted(job) {
        this.updateJobStatus(job);
        this.showStatus(`Job ${job.id} completed successfully`, 'success');
        this.loadRecentJobs();
    }

    handleJobFailed(job, error) {
        this.updateJobStatus(job);
        this.showStatus(`Job ${job.id} failed: ${error}`, 'error');
        this.loadRecentJobs();
    }

    handleAgentsList(agents) {
        console.log('Available agents:', agents);
    }

    handleJobsList(jobs) {
        this.jobs = jobs;
        this.renderJobsList();
    }

    showOutputSection() {
        const outputSection = document.querySelector('.output-section');
        outputSection.style.display = 'block';
        outputSection.scrollIntoView({ behavior: 'smooth' });
    }

    updateJobStatus(job) {
        document.getElementById('current-job-id').textContent = `Job ID: ${job.id}`;

        const statusBadge = document.querySelector('.status-badge');
        statusBadge.textContent = job.status;
        statusBadge.className = `status-badge ${job.status}`;

        if (job.result) {
            document.querySelector('.job-result').innerHTML = `
                <h3>Result</h3>
                <pre>${this.escapeHtml(JSON.stringify(job.result, null, 2))}</pre>
            `;
        }
    }

    appendLog(text) {
        if (!text) return;

        const progressLog = document.querySelector('.progress-log');
        const timestamp = new Date().toLocaleTimeString();

        const logEntry = document.createElement('div');
        logEntry.className = 'log-entry';
        logEntry.innerHTML = `<span class="timestamp">[${timestamp}]</span> ${this.escapeHtml(text)}`;

        progressLog.appendChild(logEntry);
        progressLog.scrollTop = progressLog.scrollHeight;
    }

    loadRecentJobs() {
        this.sendWebSocketMessage({ type: 'list_jobs' });
    }

    renderJobsList() {
        const jobsGrid = document.querySelector('.jobs-grid');
        jobsGrid.innerHTML = '';

        if (this.jobs.length === 0) {
            jobsGrid.innerHTML = '<p>No recent jobs</p>';
            return;
        }

        this.jobs.slice(0, 10).forEach(job => {
            const jobCard = document.createElement('div');
            jobCard.className = 'job-card';
            jobCard.innerHTML = `
                <div class="job-card-header">
                    <span class="job-card-type">${job.agent_type}</span>
                    <span class="job-card-status status-badge ${job.status}">${job.status}</span>
                </div>
                <div class="job-card-description">
                    ${this.escapeHtml(this.truncate(job.description || 'No description', 100))}
                </div>
                <div class="job-id">${job.id}</div>
            `;

            jobCard.addEventListener('click', () => {
                this.loadJobDetails(job.id);
            });

            jobsGrid.appendChild(jobCard);
        });
    }

    loadJobDetails(jobId) {
        this.sendWebSocketMessage({
            type: 'get_job',
            job_id: jobId
        });
    }

    // ========== Utility Functions ==========

    showStatus(message, type = 'info') {
        // Create a temporary status notification
        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.textContent = message;
        notification.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 1rem 1.5rem;
            background: ${type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : type === 'warning' ? '#f59e0b' : '#2563eb'};
            color: white;
            border-radius: 8px;
            box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.1);
            z-index: 1000;
            animation: slideIn 0.3s ease-out;
        `;

        document.body.appendChild(notification);

        setTimeout(() => {
            notification.style.animation = 'slideOut 0.3s ease-out';
            setTimeout(() => notification.remove(), 300);
        }, 3000);
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    truncate(text, maxLength) {
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    }
}

// Add animation keyframes
const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from {
            transform: translateX(100%);
            opacity: 0;
        }
        to {
            transform: translateX(0);
            opacity: 1;
        }
    }

    @keyframes slideOut {
        from {
            transform: translateX(0);
            opacity: 1;
        }
        to {
            transform: translateX(100%);
            opacity: 0;
        }
    }
`;
document.head.appendChild(style);

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.pedroCLI = new PedroCLIApp();
});
