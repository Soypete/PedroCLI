// Voice Recording Manager for PedroCLI
const VoiceRecorder = {
    mediaRecorder: null,
    audioChunks: [],
    isRecording: false,
    stream: null,
    activeButtonId: null,  // Track which button started the recording

    // Initialize and request microphone permission
    async init() {
        try {
            // Check if browser supports getUserMedia
            if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                throw new Error('Your browser does not support audio recording');
            }

            // Request microphone access
            this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            console.log('Voice: Microphone access granted');
            return true;
        } catch (error) {
            console.error('Voice: Failed to initialize:', error);
            alert('Failed to access microphone: ' + error.message);
            return false;
        }
    },

    // Start recording
    async startRecording(buttonId) {
        if (this.isRecording) {
            console.warn('Voice: Already recording');
            return;
        }

        // Check if stream exists and has active tracks
        const needsInit = !this.stream ||
            !this.stream.active ||
            this.stream.getTracks().every(t => t.readyState === 'ended');

        if (needsInit) {
            // Release old stream if exists
            if (this.stream) {
                this.stream.getTracks().forEach(t => t.stop());
                this.stream = null;
            }
            const initialized = await this.init();
            if (!initialized) {
                return;
            }
        }

        try {
            // Create MediaRecorder with explicit mime type
            this.audioChunks = [];
            const options = MediaRecorder.isTypeSupported('audio/webm')
                ? { mimeType: 'audio/webm' }
                : {};
            this.mediaRecorder = new MediaRecorder(this.stream, options);

            this.mediaRecorder.ondataavailable = (event) => {
                if (event.data.size > 0) {
                    this.audioChunks.push(event.data);
                }
            };

            this.mediaRecorder.onstop = () => {
                console.log('Voice: Recording stopped');
                this.isRecording = false;
            };

            this.mediaRecorder.start();
            this.isRecording = true;
            console.log('Voice: Recording started');

            // Update UI
            this.updateUI(true, buttonId);
        } catch (error) {
            console.error('Voice: Failed to start recording:', error);
            alert('Failed to start recording: ' + error.message);
        }
    },

    // Stop recording and return audio blob
    stopRecording() {
        return new Promise((resolve, reject) => {
            if (!this.isRecording || !this.mediaRecorder) {
                reject(new Error('Not currently recording'));
                return;
            }

            this.mediaRecorder.onstop = () => {
                const audioBlob = new Blob(this.audioChunks, { type: 'audio/webm' });
                console.log('Voice: Recording stopped, blob size:', audioBlob.size);
                this.isRecording = false;
                this.updateUI(false);
                resolve(audioBlob);
            };

            this.mediaRecorder.stop();
        });
    },

    // Transcribe audio and fill target input
    async transcribeAndFill(targetInputId) {
        try {
            // Stop recording
            const audioBlob = await this.stopRecording();

            // Show loading state
            const targetInput = document.getElementById(targetInputId);
            const originalPlaceholder = targetInput.placeholder;
            targetInput.placeholder = 'Transcribing...';
            targetInput.disabled = true;

            // Create form data
            const formData = new FormData();
            formData.append('audio', audioBlob, 'recording.webm');

            // Send to server
            const response = await fetch('/api/voice/transcribe', {
                method: 'POST',
                body: formData
            });

            const result = await response.json();

            if (result.success && result.text) {
                // Fill the input with transcribed text
                targetInput.value = result.text;
                console.log('Voice: Transcription successful:', result.text);
            } else {
                throw new Error(result.error || 'Transcription failed');
            }

            // Restore input state
            targetInput.placeholder = originalPlaceholder;
            targetInput.disabled = false;
            targetInput.focus();
        } catch (error) {
            console.error('Voice: Transcription failed:', error);
            alert('Transcription failed: ' + error.message);

            // Restore input state
            const targetInput = document.getElementById(targetInputId);
            if (targetInput) {
                targetInput.placeholder = 'Describe what you want to build...';
                targetInput.disabled = false;
            }
        }
    },

    // Update UI to show recording state
    updateUI(isRecording, buttonId) {
        // Use provided buttonId, or fall back to activeButtonId, or default to 'voice-btn'
        const btnId = buttonId || this.activeButtonId || 'voice-btn';
        const voiceBtn = document.getElementById(btnId);
        if (!voiceBtn) return;

        if (isRecording) {
            voiceBtn.classList.add('recording');
            voiceBtn.innerHTML = `
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                    <rect x="7" y="7" width="6" height="6" rx="1"/>
                </svg>
                <span class="ml-2">Stop</span>
            `;
            voiceBtn.classList.remove('bg-gray-200', 'hover:bg-gray-300');
            voiceBtn.classList.add('bg-red-500', 'hover:bg-red-600', 'text-white', 'animate-pulse');
        } else {
            voiceBtn.classList.remove('recording');
            voiceBtn.innerHTML = `
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M7 4a3 3 0 016 0v6a3 3 0 11-6 0V4z"/>
                    <path d="M5.5 9.643a.75.75 0 00-1.5 0V10c0 3.06 2.29 5.585 5.25 5.954V17.5h-1.5a.75.75 0 000 1.5h4.5a.75.75 0 000-1.5h-1.5v-1.546A6.001 6.001 0 0016 10v-.357a.75.75 0 00-1.5 0V10a4.5 4.5 0 01-9 0v-.357z"/>
                </svg>
                <span class="ml-2">Voice</span>
            `;
            voiceBtn.classList.remove('bg-red-500', 'hover:bg-red-600', 'text-white', 'animate-pulse');
            voiceBtn.classList.add('bg-gray-200', 'hover:bg-gray-300');
        }
    },

    // Toggle recording (start if stopped, transcribe if recording)
    async toggle(targetInputId, buttonId) {
        // Determine button ID from targetInputId if not provided
        if (!buttonId) {
            // Map input IDs to button IDs
            const buttonMap = {
                'description': 'voice-btn',
                'topic': 'voice-btn-topic',
                'notes': 'voice-btn-notes',
                'guest-bio': 'voice-btn-bio'
            };
            buttonId = buttonMap[targetInputId] || 'voice-btn';
        }

        if (this.isRecording) {
            // Stop and transcribe
            await this.transcribeAndFill(targetInputId);
        } else {
            // Start recording - track which button
            this.activeButtonId = buttonId;
            await this.startRecording(buttonId);
        }
    },

    // Check if voice is enabled on server
    async checkStatus() {
        try {
            const response = await fetch('/api/voice/status');
            const status = await response.json();
            return status.running;
        } catch (error) {
            console.error('Voice: Status check failed:', error);
            return false;
        }
    },

    // Cleanup - stop recording and release microphone
    cleanup() {
        if (this.mediaRecorder && this.isRecording) {
            this.mediaRecorder.stop();
        }

        if (this.stream) {
            this.stream.getTracks().forEach(track => track.stop());
            this.stream = null;
        }

        this.isRecording = false;
        console.log('Voice: Cleanup complete');
    }
};

// Initialize voice status on page load
document.addEventListener('DOMContentLoaded', async function() {
    // Find all voice buttons
    const voiceBtns = document.querySelectorAll('.voice-btn, #voice-btn');
    if (voiceBtns.length === 0) return;

    // Check if voice is enabled
    const isEnabled = await VoiceRecorder.checkStatus();
    if (!isEnabled) {
        voiceBtns.forEach(btn => {
            btn.disabled = true;
            btn.title = 'Voice transcription is not enabled. Start whisper.cpp server to use this feature.';
            btn.classList.add('opacity-50', 'cursor-not-allowed');
        });
        console.log('Voice: Disabled (whisper.cpp not running)');
    } else {
        console.log('Voice: Enabled');
    }
});

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    VoiceRecorder.cleanup();
});
