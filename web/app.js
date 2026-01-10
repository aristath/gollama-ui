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
        currentModel = e.target.value || null;
        updateSendButtonState();
        updateUnloadButtonState();
        if (!currentModel) {
            addSystemMessage('Please select a model to start chatting.');
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
        const response = await fetch('/api/models');
        if (!response.ok) {
            throw new Error(`Failed to load models: ${response.statusText}`);
        }
        
        const data = await response.json();
        models = data.models || [];
        
        modelSelect.innerHTML = '<option value="">Select a model...</option>';
        
        if (models.length === 0) {
            modelSelect.innerHTML = '<option value="">No models available</option>';
            addSystemMessage('No models found. Please ensure Ollama is running and has models installed.');
            return;
        }
        
        models.forEach(model => {
            const option = document.createElement('option');
            option.value = model.name;
            option.textContent = model.name;
            modelSelect.appendChild(option);
        });
        
        // Auto-select first model
        if (models.length > 0) {
            modelSelect.value = models[0].name;
            currentModel = models[0].name;
            updateSendButtonState();
            updateUnloadButtonState();
        }
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