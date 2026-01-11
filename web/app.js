// State
let models = [];
let currentModel = null;
let conversationHistory = [];
let isStreaming = false;
let currentStreamController = null;

// DOM elements
const modelSelect = document.getElementById('model-select');
const messageInput = document.getElementById('message-input');
const sendButton = document.getElementById('send-button');
const unloadButton = document.getElementById('unload-button');
const messagesContainer = document.getElementById('messages');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadModels();

    modelSelect.addEventListener('change', (e) => {
        const selectedModel = e.target.value;
        if (!selectedModel) {
            currentModel = null;
            updateSendButtonState();
            updateUnloadButtonState();
            addSystemMessage('Please select a model to start chatting.');
            return;
        }

        // If selecting a different model, load it
        if (selectedModel !== currentModel) {
            switchModel(selectedModel);
        }
    });

    sendButton.addEventListener('click', sendMessage);
    unloadButton.addEventListener('click', unloadModel);
    
    messageInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            if (!sendButton.disabled) {
                sendMessage();
            }
        }
    });
});

// Load available models
async function loadModels() {
    try {
        // Fetch all available models and current status
        const response = await fetch('/api/models/status');
        if (!response.ok) {
            throw new Error(`Failed to load models: ${response.statusText}`);
        }

        const data = await response.json();
        models = data.available_models || [];
        const currentLoadedModel = data.current_model;

        modelSelect.innerHTML = '<option value="">Select a model...</option>';

        if (models.length === 0) {
            modelSelect.innerHTML = '<option value="">No models available</option>';
            addSystemMessage('No models found. Please ensure models are available.');
            return;
        }

        models.forEach(modelName => {
            const option = document.createElement('option');
            option.value = modelName;
            option.textContent = modelName;
            modelSelect.appendChild(option);
        });

        // Set current model as selected
        if (currentLoadedModel) {
            modelSelect.value = currentLoadedModel;
            currentModel = currentLoadedModel;
        } else if (models.length > 0) {
            // Fallback to first model if no current model is set
            modelSelect.value = models[0];
            currentModel = models[0];
        }

        updateSendButtonState();
        updateUnloadButtonState();
    } catch (error) {
        console.error('Error loading models:', error);
        addErrorMessage(`Failed to load models: ${error.message}`);
        modelSelect.innerHTML = '<option value="">Error loading models</option>';
    }
}

// Send message
async function sendMessage() {
    const message = messageInput.value.trim();
    
    if (!message || !currentModel || isStreaming) {
        return;
    }
    
    // Add user message to UI
    addMessage('user', message);
    conversationHistory.push({ role: 'user', content: message });
    
    // Clear input and disable
    messageInput.value = '';
    messageInput.disabled = true;
    sendButton.disabled = true;
    isStreaming = true;
    
    // Create assistant message placeholder
    const assistantMessageId = addMessage('assistant', '', true);
    const assistantMessageEl = document.getElementById(assistantMessageId);
    const contentEl = assistantMessageEl.querySelector('.content');
    
    try {
        // Create abort controller for cancellation
        currentStreamController = new AbortController();
        
        const response = await fetch('/api/chat', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                model: currentModel,
                messages: conversationHistory,
                stream: true,
            }),
            signal: currentStreamController.signal,
        });
        
        if (!response.ok) {
            throw new Error(`Chat request failed: ${response.statusText}`);
        }
        
        // Read streaming response
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let assistantContent = '';
        
        while (true) {
            const { done, value } = await reader.read();
            
            if (done) {
                break;
            }
            
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop(); // Keep incomplete line in buffer
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const data = JSON.parse(line.slice(6));
                        
                        if (data.error) {
                            throw new Error(data.error);
                        }
                        
                        if (data.message && data.message.content) {
                            assistantContent += data.message.content;
                            contentEl.textContent = assistantContent;
                            scrollToBottom();
                        }
                        
                        if (data.done) {
                            // Update conversation history with complete message
                            conversationHistory.push({ role: 'assistant', content: assistantContent });
                            assistantMessageEl.classList.remove('streaming');
                            break;
                        }
                    } catch (e) {
                        console.error('Error parsing stream data:', e);
                    }
                }
            }
        }
        
        if (assistantContent === '') {
            contentEl.textContent = '(No response)';
        }
        
    } catch (error) {
        if (error.name === 'AbortError') {
            contentEl.textContent = '(Cancelled)';
        } else {
            console.error('Error sending message:', error);
            contentEl.textContent = `Error: ${error.message}`;
            assistantMessageEl.classList.add('error');
        }
    } finally {
        isStreaming = false;
        currentStreamController = null;
        messageInput.disabled = false;
        updateSendButtonState();
        updateUnloadButtonState();
        messageInput.focus();
        
        if (assistantMessageEl.classList.contains('streaming')) {
            assistantMessageEl.classList.remove('streaming');
        }
    }
}

// Add message to UI
function addMessage(role, content, streaming = false) {
    const messageId = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    const messageEl = document.createElement('div');
    messageEl.id = messageId;
    messageEl.className = `message ${role}${streaming ? ' streaming' : ''}`;
    
    messageEl.innerHTML = `
        <div class="role">${role}</div>
        <div class="content">${escapeHtml(content)}</div>
    `;
    
    messagesContainer.appendChild(messageEl);
    scrollToBottom();
    
    return messageId;
}

// Add system/error message
function addSystemMessage(content) {
    const messageEl = document.createElement('div');
    messageEl.className = 'message assistant';
    messageEl.innerHTML = `
        <div class="role">system</div>
        <div class="content">${escapeHtml(content)}</div>
    `;
    messagesContainer.appendChild(messageEl);
    scrollToBottom();
}

function addErrorMessage(content) {
    const errorEl = document.createElement('div');
    errorEl.className = 'error';
    errorEl.textContent = content;
    messagesContainer.appendChild(errorEl);
    scrollToBottom();
}

// Unload model from memory
async function unloadModel() {
    if (!currentModel || isStreaming) {
        return;
    }

    if (!confirm(`Unload ${currentModel} from memory?`)) {
        return;
    }

    unloadButton.disabled = true;
    unloadButton.textContent = 'Unloading...';

    try {
        const response = await fetch(`/api/models/${encodeURIComponent(currentModel)}/unload`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        if (!response.ok) {
            const errorData = await response.text();
            throw new Error(`Failed to unload model: ${errorData || response.statusText}`);
        }

        const data = await response.json();
        addSystemMessage(data.message || `Model ${currentModel} unloaded from memory.`);
        
    } catch (error) {
        console.error('Error unloading model:', error);
        addErrorMessage(`Failed to unload model: ${error.message}`);
    } finally {
        unloadButton.textContent = 'Unload';
        updateUnloadButtonState();
    }
}

// Update send button state
function updateSendButtonState() {
    sendButton.disabled = !currentModel || isStreaming || messageInput.value.trim() === '';
}

// Update unload button state
function updateUnloadButtonState() {
    unloadButton.disabled = !currentModel || isStreaming;
}

// Enable send button when typing
messageInput.addEventListener('input', () => {
    updateSendButtonState();
});

// Scroll to bottom
function scrollToBottom() {
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Model Switching Functions
let loadingInterval = null;

// Open model switcher panel
function openModelSwitcher() {
    const switcher = document.getElementById('model-switcher');
    switcher.classList.remove('hidden');
    loadAvailableModels();
}

// Close model switcher panel
function closeModelSwitcher() {
    const switcher = document.getElementById('model-switcher');
    switcher.classList.add('hidden');
}

// Load available models and populate the switcher
async function loadAvailableModels() {
    try {
        const response = await fetch('/api/models/available');
        if (!response.ok) {
            throw new Error(`Failed to load models: ${response.statusText}`);
        }

        const data = await response.json();
        const models = data.models || [];

        const modelsList = document.getElementById('available-models-list');
        modelsList.innerHTML = '';

        if (models.length === 0) {
            modelsList.innerHTML = '<p>No models found</p>';
            return;
        }

        // Get current model for highlighting
        const currentModel = modelSelect.value;

        models.forEach(model => {
            const modelItem = document.createElement('div');
            modelItem.className = 'model-item';
            if (model.name === currentModel) {
                modelItem.classList.add('current');
            }

            // Format file size
            const sizeGb = (model.size / (1024 * 1024 * 1024)).toFixed(2);
            const sizeText = `${sizeGb} GB`;

            modelItem.innerHTML = `
                <div>${model.name}</div>
                <div class="model-item size">${sizeText}</div>
            `;

            modelItem.addEventListener('click', () => switchModel(model.name));
            modelsList.appendChild(modelItem);
        });

    } catch (error) {
        console.error('Error loading models:', error);
        const modelsList = document.getElementById('available-models-list');
        modelsList.innerHTML = `<p style="color: #ff7c7c;">Error loading models: ${error.message}</p>`;
    }
}

// Switch to a different model
async function switchModel(modelName) {
    if (modelName === currentModel) {
        showStatusMessage(`Model ${modelName} is already loaded`, 'info');
        return;
    }

    // Disable chat during model switching
    messageInput.disabled = true;
    sendButton.disabled = true;
    modelSelect.disabled = true;

    // Show loading indicator
    const loadingIndicator = document.getElementById('loading-indicator');
    loadingIndicator.classList.remove('hidden');

    // Start timer
    let seconds = 0;
    loadingInterval = setInterval(() => {
        seconds++;
        document.querySelector('.loading-timer').textContent = `${seconds}s elapsed`;

        // Simulate progress (cap at 90%)
        const progress = Math.min(90, seconds * 1.5);
        document.querySelector('.progress-fill').style.width = `${progress}%`;
    }, 1000);

    try {
        const response = await fetch(`/api/models/${encodeURIComponent(modelName)}/load`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || response.statusText);
        }

        const data = await response.json();

        // Stop timer and show 100%
        clearInterval(loadingInterval);
        document.querySelector('.progress-fill').style.width = '100%';

        if (data.success) {
            // Update current model
            currentModel = modelName;
            modelSelect.value = modelName;
            updateSendButtonState();
            updateUnloadButtonState();

            showStatusMessage(`✅ ${modelName} loaded in ${data.time_taken}`, 'success');

            // Reload available models to update UI
            setTimeout(() => {
                loadAvailableModels();
            }, 500);
        } else {
            throw new Error(data.message || 'Failed to load model');
        }

    } catch (error) {
        console.error('Error switching model:', error);
        showStatusMessage(`❌ Failed to load ${modelName}: ${error.message}`, 'error');
    } finally {
        // Hide loading indicator
        setTimeout(() => {
            const loadingIndicator = document.getElementById('loading-indicator');
            loadingIndicator.classList.add('hidden');
            document.querySelector('.progress-fill').style.width = '0%';
            clearInterval(loadingInterval);

            // Re-enable chat
            messageInput.disabled = false;
            sendButton.disabled = false;
            modelSelect.disabled = false;
            updateSendButtonState();
        }, 1000);
    }
}

// Show status message
function showStatusMessage(message, type = 'info') {
    const statusMsg = document.getElementById('status-message');
    statusMsg.textContent = message;
    statusMsg.className = `status-message ${type}`;
    statusMsg.classList.remove('hidden');

    // Auto-hide after 5 seconds
    setTimeout(() => {
        statusMsg.classList.add('hidden');
    }, 5000);
}

// Setup event listeners for model switcher and settings
document.addEventListener('DOMContentLoaded', () => {
    // Model switcher
    const switchBtn = document.getElementById('switch-model-btn');
    if (switchBtn) {
        switchBtn.addEventListener('click', openModelSwitcher);
    }

    const closeBtn = document.getElementById('switcher-close');
    if (closeBtn) {
        closeBtn.addEventListener('click', closeModelSwitcher);
    }

    // Settings panel
    const settingsBtn = document.getElementById('settings-btn');
    if (settingsBtn) {
        settingsBtn.addEventListener('click', openSettingsPanel);
    }

    const settingsClose = document.getElementById('settings-close');
    if (settingsClose) {
        settingsClose.addEventListener('click', closeSettingsPanel);
    }

    const feedsSaveBtn = document.getElementById('feeds-save-btn');
    if (feedsSaveBtn) {
        feedsSaveBtn.addEventListener('click', saveCustomFeeds);
    }

    const feedsResetBtn = document.getElementById('feeds-reset-btn');
    if (feedsResetBtn) {
        feedsResetBtn.addEventListener('click', resetFeeds);
    }

    // Chat timeout control
    const chatTimeoutInput = document.getElementById('chat-timeout-input');
    if (chatTimeoutInput) {
        chatTimeoutInput.addEventListener('input', updateTimeoutDisplay);
    }

    const timeoutSaveBtn = document.getElementById('timeout-save-btn');
    if (timeoutSaveBtn) {
        timeoutSaveBtn.addEventListener('click', saveChatTimeout);
    }

    // Tool settings toggles
    const toolWebSearch = document.getElementById('tool-web-search');
    if (toolWebSearch) {
        toolWebSearch.addEventListener('change', saveToolSettings);
    }

    const toolFeeds = document.getElementById('tool-feeds');
    if (toolFeeds) {
        toolFeeds.addEventListener('change', saveToolSettings);
    }

    const toolSentinel = document.getElementById('tool-sentinel');
    if (toolSentinel) {
        toolSentinel.addEventListener('change', saveToolSettings);
    }

    // Load feeds on panel open
    loadAndDisplayFeeds();
    loadToolSettings();
    loadChatTimeout();
});

// Parse duration string (e.g., "5m", "1h", "30m", "72h") into seconds
function parseDurationToSeconds(durationStr) {
    if (!durationStr || typeof durationStr !== 'string') {
        return null;
    }

    durationStr = durationStr.trim().toLowerCase();

    // Match pattern: number + unit (s, m, h, d)
    const match = durationStr.match(/^(\d+(?:\.\d+)?)\s*([smhd])$/);
    if (!match) {
        return null;
    }

    const value = parseFloat(match[1]);
    const unit = match[2];

    let seconds;
    switch (unit) {
        case 's':
            seconds = value;
            break;
        case 'm':
            seconds = value * 60;
            break;
        case 'h':
            seconds = value * 3600;
            break;
        case 'd':
            seconds = value * 86400;
            break;
        default:
            return null;
    }

    // Validate range (1 second to 30 days)
    if (seconds < 1 || seconds > 2592000) {
        return null;
    }

    return Math.round(seconds);
}

// Convert seconds to human-readable format
function formatSecondsToDisplay(seconds) {
    if (seconds < 60) {
        return `${seconds} second${seconds !== 1 ? 's' : ''}`;
    } else if (seconds < 3600) {
        const minutes = Math.round(seconds / 60);
        return `${minutes} minute${minutes !== 1 ? 's' : ''}`;
    } else if (seconds < 86400) {
        const hours = Math.round(seconds / 3600);
        return `${hours} hour${hours !== 1 ? 's' : ''}`;
    } else {
        const days = Math.round(seconds / 86400);
        return `${days} day${days !== 1 ? 's' : ''}`;
    }
}

// Convert seconds to readable duration string (e.g., "1h 30m")
function formatSecondsToDuration(seconds) {
    if (seconds < 60) {
        return `${seconds}s`;
    } else if (seconds < 3600) {
        const minutes = Math.round(seconds / 60);
        return `${minutes}m`;
    } else if (seconds < 86400) {
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.round((seconds % 3600) / 60);
        if (minutes === 0) {
            return `${hours}h`;
        }
        return `${hours}h ${minutes}m`;
    } else {
        const days = Math.floor(seconds / 86400);
        const hours = Math.round((seconds % 86400) / 3600);
        if (hours === 0) {
            return `${days}d`;
        }
        return `${days}d ${hours}h`;
    }
}

function updateTimeoutDisplay() {
    const input = document.getElementById('chat-timeout-input');
    const display = document.getElementById('timeout-display');
    const statusEl = document.getElementById('timeout-status');

    if (input && display) {
        const inputValue = input.value.trim();

        if (!inputValue) {
            display.textContent = '';
            display.classList.remove('valid', 'invalid');
            return;
        }

        const seconds = parseDurationToSeconds(inputValue);

        if (seconds === null) {
            display.textContent = 'Invalid format (use: 5m, 1h, 72h, etc.)';
            display.classList.remove('valid');
            display.classList.add('invalid');
        } else {
            const displayText = formatSecondsToDisplay(seconds);
            display.textContent = displayText;
            display.classList.remove('invalid');
            display.classList.add('valid');
        }
    }
}

async function loadChatTimeout() {
    try {
        const response = await fetch('/api/settings/chat-timeout');
        if (!response.ok) {
            console.warn('Failed to load chat timeout settings');
            return;
        }

        const data = await response.json();
        const input = document.getElementById('chat-timeout-input');
        if (input) {
            const seconds = data.timeout_seconds || 300;
            // Display as readable duration string (e.g., "5m", "1h")
            input.value = formatSecondsToDuration(seconds);
            updateTimeoutDisplay();
        }
    } catch (error) {
        console.error('Error loading chat timeout:', error);
    }
}

async function saveChatTimeout() {
    try {
        const input = document.getElementById('chat-timeout-input');
        const inputValue = input.value.trim();

        // Parse duration string into seconds
        const timeoutSeconds = parseDurationToSeconds(inputValue);

        if (timeoutSeconds === null) {
            const statusEl = document.getElementById('timeout-status');
            if (statusEl) {
                statusEl.textContent = '✗ Invalid format. Use: 5m, 1h, 72h, etc.';
                statusEl.classList.add('error');
                statusEl.classList.remove('success');
            }
            return;
        }

        const response = await fetch('/api/settings/chat-timeout', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ timeout_seconds: timeoutSeconds }),
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error || response.statusText);
        }

        const data = await response.json();

        // Update input to normalized format
        input.value = formatSecondsToDuration(data.timeout_seconds);

        // Show success message
        const statusEl = document.getElementById('timeout-status');
        if (statusEl) {
            const displayText = formatSecondsToDisplay(data.timeout_seconds);
            statusEl.textContent = `✓ Chat timeout updated to ${displayText}`;
            statusEl.classList.add('success');
            statusEl.classList.remove('error');

            setTimeout(() => {
                statusEl.textContent = '';
                statusEl.classList.remove('success');
            }, 3000);
        }
    } catch (error) {
        console.error('Error saving chat timeout:', error);
        const statusEl = document.getElementById('timeout-status');
        if (statusEl) {
            statusEl.textContent = `✗ Error: ${error.message}`;
            statusEl.classList.add('error');
            statusEl.classList.remove('success');
        }
    }
}

// Settings Panel Functions
function openSettingsPanel() {
    const panel = document.getElementById('settings-panel');
    if (panel) {
        panel.classList.remove('hidden');
        loadAndDisplayFeeds();
        loadToolSettings();
        loadChatTimeout();
    }
}

function closeSettingsPanel() {
    const panel = document.getElementById('settings-panel');
    if (panel) {
        panel.classList.add('hidden');
    }
}

async function loadAndDisplayFeeds() {
    try {
        const response = await fetch('/api/settings/feeds');
        if (!response.ok) {
            throw new Error('Failed to load feeds');
        }

        const data = await response.json();
        const feeds = data.feeds || {};

        // Display feeds list
        const feedsList = document.getElementById('feeds-list');
        if (feedsList) {
            if (Object.keys(feeds).length === 0) {
                feedsList.innerHTML = '<p>No feeds configured</p>';
            } else {
                let html = '';
                for (const [topic, url] of Object.entries(feeds)) {
                    html += `
                        <div class="feed-item">
                            <div class="feed-name">${escapeHtml(topic)}</div>
                            <div class="feed-url">${escapeHtml(url)}</div>
                        </div>
                    `;
                }
                feedsList.innerHTML = html;
            }
        }

        // Load custom feeds into textarea (for editing)
        const feedsInput = document.getElementById('feeds-input');
        if (feedsInput) {
            const customFeeds = {};
            // Since we can't distinguish custom vs default from the response,
            // we could enhance this later. For now, show all feeds.
            feedsInput.value = Object.entries(feeds)
                .map(([topic, url]) => `${topic}=${url}`)
                .join('\n');
        }
    } catch (error) {
        console.error('Error loading feeds:', error);
        const feedsList = document.getElementById('feeds-list');
        if (feedsList) {
            feedsList.innerHTML = `<p style="color: #ff7c7c;">Error loading feeds: ${error.message}</p>`;
        }
    }
}

async function saveCustomFeeds() {
    const feedsInput = document.getElementById('feeds-input');
    if (!feedsInput) return;

    const feedsText = feedsInput.value.trim();
    const feeds = {};

    // Parse feeds from textarea
    const lines = feedsText.split(/[\n,]+/).filter(line => line.trim());
    for (const line of lines) {
        const parts = line.trim().split('=');
        if (parts.length === 2) {
            const topic = parts[0].trim();
            const url = parts[1].trim();
            if (topic && url) {
                feeds[topic] = url;
            }
        }
    }

    try {
        const response = await fetch('/api/settings/feeds', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ feeds }),
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error || response.statusText);
        }

        const data = await response.json();

        // Show success message
        const statusEl = document.getElementById('feeds-status');
        if (statusEl) {
            statusEl.textContent = `✓ Successfully saved ${data.count} feed(s)`;
            statusEl.classList.add('success');
            statusEl.classList.remove('error');

            setTimeout(() => {
                statusEl.textContent = '';
                statusEl.classList.remove('success');
            }, 3000);
        }

        // Reload feeds display
        loadAndDisplayFeeds();
    } catch (error) {
        console.error('Error saving feeds:', error);
        const statusEl = document.getElementById('feeds-status');
        if (statusEl) {
            statusEl.textContent = `✗ Error: ${error.message}`;
            statusEl.classList.add('error');
            statusEl.classList.remove('success');
        }
    }
}

function resetFeeds() {
    const feedsInput = document.getElementById('feeds-input');
    if (feedsInput) {
        feedsInput.value = '';

        const statusEl = document.getElementById('feeds-status');
        if (statusEl) {
            statusEl.textContent = 'Cleared custom feeds. Click Save to apply.';
            statusEl.classList.add('error');
            statusEl.classList.remove('success');
        }
    }
}

// Tool Settings Functions
async function loadToolSettings() {
    try {
        const response = await fetch('/api/settings/tools');
        if (!response.ok) {
            console.warn('Failed to load tool settings');
            return;
        }

        const data = await response.json();
        const toolWebSearch = document.getElementById('tool-web-search');
        const toolFeeds = document.getElementById('tool-feeds');
        const toolSentinel = document.getElementById('tool-sentinel');

        if (toolWebSearch) {
            toolWebSearch.checked = data.enable_web_search || false;
        }

        if (toolFeeds) {
            toolFeeds.checked = data.enable_feeds || false;
        }

        if (toolSentinel) {
            toolSentinel.checked = data.enable_sentinel || false;
        }
    } catch (error) {
        console.error('Error loading tool settings:', error);
    }
}

async function saveToolSettings() {
    try {
        const toolWebSearch = document.getElementById('tool-web-search');
        const toolFeeds = document.getElementById('tool-feeds');
        const toolSentinel = document.getElementById('tool-sentinel');

        const settings = {
            enable_web_search: toolWebSearch ? toolWebSearch.checked : false,
            enable_feeds: toolFeeds ? toolFeeds.checked : false,
            enable_sentinel: toolSentinel ? toolSentinel.checked : false,
        };

        const response = await fetch('/api/settings/tools', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(settings),
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error || response.statusText);
        }

        const data = await response.json();

        // Show success message
        const statusEl = document.getElementById('tools-status');
        if (statusEl) {
            const toolsList = [];
            if (data.enable_web_search) toolsList.push('Web Search');
            if (data.enable_feeds) toolsList.push('News Feeds');
            const toolsText = toolsList.length > 0 ? toolsList.join(', ') : 'No tools enabled';

            statusEl.textContent = `✓ Tools updated: ${toolsText}`;
            statusEl.classList.add('success');
            statusEl.classList.remove('error');

            setTimeout(() => {
                statusEl.textContent = '';
                statusEl.classList.remove('success');
            }, 3000);
        }
    } catch (error) {
        console.error('Error saving tool settings:', error);
        const statusEl = document.getElementById('tools-status');
        if (statusEl) {
            statusEl.textContent = `✗ Error: ${error.message}`;
            statusEl.classList.add('error');
            statusEl.classList.remove('success');
        }
    }
}
