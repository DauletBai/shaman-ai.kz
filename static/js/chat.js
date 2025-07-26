// static/js/chat.js
// --- DOM элементы ---
const chatBox = document.getElementById('chat-box');
const chatForm = document.getElementById('chat-form');
const userInput = document.getElementById('user-input');
const sendButton = document.getElementById('send-button');
const loadingIndicator = document.getElementById('loading-indicator');
const recordButton = document.getElementById('record-button');
const micIcon = document.getElementById('mic-icon');
const speechError = document.getElementById('speech-error');
const sessionHistoryList = document.getElementById('session-history');
const newChatButton = document.getElementById('new-chat-btn');
const currentChatTitleElement = document.getElementById('current-chat-title');

const attachDocButton = document.getElementById('attach-doc-btn');
const attachImgButton = document.getElementById('attach-img-btn');
const fileInputDoc = document.getElementById('file-input-doc');
const fileInputImg = document.getElementById('file-input-img');
const attachmentPreviewArea = document.getElementById('attachment-preview-area');


// --- Глобальные переменные состояния чата ---
let currentChatSessionUUID = null;
let isChatLoading = false;
let activeSessionsCache = [];
let attachedFile = null; 

// --- Инициализация SpeechRecognition (STT) ---
const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
let recognition;
let isRecording = false;
let accumulatedTranscript = '';

if (SpeechRecognition) {
    recognition = new SpeechRecognition();
    recognition.continuous = true;
    recognition.lang = 'ru-RU';
    recognition.interimResults = true;

    recognition.onstart = () => {
        isRecording = true;
        accumulatedTranscript = userInput.value;
        if (micIcon) micIcon.classList.replace('bi-mic-fill', 'bi-stop-circle-fill');
        if (recordButton) {
            recordButton.classList.remove('btn-outline-secondary');
            recordButton.classList.remove('btn-success');
            recordButton.classList.add('btn-danger');
            recordButton.setAttribute('aria-label', 'Остановить голосовой ввод');
            recordButton.setAttribute('aria-pressed', 'true');
        }
        if (speechError) speechError.style.display = 'none';
        console.log("Распознавание речи начато (непрерывный)");
    };

    recognition.onresult = (event) => {
        let interim_transcript = '';
        let final_piece_this_event = '';
        for (let i = event.resultIndex; i < event.results.length; ++i) {
            const transcript_part = event.results[i][0].transcript;
            if (event.results[i].isFinal) {
                final_piece_this_event += transcript_part.trim() + ' ';
            } else {
                interim_transcript += transcript_part;
            }
        }
        if (final_piece_this_event) {
            accumulatedTranscript = (accumulatedTranscript ? accumulatedTranscript.trim() + ' ' : '') + final_piece_this_event.trim();
        }
        userInput.value = (accumulatedTranscript + interim_transcript).trim();
        if (typeof OnInput === "function") OnInput.call(userInput);
    };

    recognition.onend = () => {
        isRecording = false;
        if (micIcon) micIcon.classList.replace('bi-stop-circle-fill', 'bi-mic-fill');
        if (recordButton) {
            recordButton.classList.add('btn-success');
            recordButton.classList.remove('btn-danger');
            recordButton.setAttribute('aria-label', 'Начать голосовой ввод');
            recordButton.setAttribute('aria-pressed', 'false');
        }
        userInput.value = accumulatedTranscript.trim();
        if (typeof OnInput === "function") OnInput.call(userInput);
        console.log("Распознавание речи остановлено. Финальный текст:", accumulatedTranscript);
        if (recognition && !recognition.errorOccurred) {
             // accumulatedTranscript = ''; // Не очищаем, чтобы текст остался в поле для возможной отправки
        }
        recognition.errorOccurred = false;
    };

    recognition.onerror = (event) => {
        console.error("Ошибка распознавания:", event.error, event.message);
        recognition.errorOccurred = true;
         if (isRecording) {
            isRecording = false;
            if (micIcon) micIcon.classList.replace('bi-stop-circle-fill', 'bi-mic-fill');
            if (recordButton) {
                recordButton.classList.add('btn-success');
                recordButton.classList.remove('btn-danger');
                recordButton.setAttribute('aria-label', 'Начать голосовой ввод');
                recordButton.setAttribute('aria-pressed', 'false');
            }
        }

        let errorMessage = `Ошибка распознавания: ${event.error}`;
        if (event.error === 'no-speech') errorMessage = 'Речь не распознана. Попробуйте еще раз.';
        else if (event.error === 'audio-capture') errorMessage = 'Ошибка захвата аудио. Проверьте микрофон.';
        else if (event.error === 'not-allowed') errorMessage = 'Доступ к микрофону запрещен.';

        if (speechError) {
             speechError.textContent = errorMessage;
             speechError.style.display = 'block';
        }
    };
} else {
    console.warn("Speech Recognition API не поддерживается.");
    if (recordButton) { recordButton.disabled = true; recordButton.title = "Распознавание речи не поддерживается"; if(micIcon) micIcon.classList.add('opacity-50');}
}

if (recordButton && recognition) {
    recordButton.addEventListener('click', () => {
        if (isRecording) {
            recognition.stop();
        } else {
            if (userInput.value && accumulatedTranscript !== userInput.value.trim()) {
                accumulatedTranscript = userInput.value.trim() + ' ';
            } else if (!userInput.value) {
                accumulatedTranscript = '';
            }
            userInput.value = accumulatedTranscript;

            recognition.start();
        }
    });
}

// --- Инициализация SpeechSynthesis (TTS) ---
const synth = ('speechSynthesis' in window) ? window.speechSynthesis : null;
let currentUtterance = null;
let russianMaleVoice = null;

if (!synth) {
    console.warn("Speech Synthesis API не поддерживается.");
} else {
    function loadAndSetVoice() {
        const voices = synth.getVoices();
        if (voices.length === 0) {
            console.warn("Список голосов пуст. Повторная попытка после voiceschanged.");
            return;
        }

        console.log("Доступные голоса:", voices);

        russianMaleVoice = voices.find(voice => voice.lang === 'ru-RU' && voice.name.toLowerCase().includes('male'));

        if (!russianMaleVoice) {
            russianMaleVoice = voices.find(voice =>
                voice.lang === 'ru-RU' &&
                (
                  voice.name.toLowerCase().includes('aleksandr') ||
                  voice.name.toLowerCase().includes('alexander') ||
                  voice.name.toLowerCase().includes('pavel') ||
                  voice.name.toLowerCase().includes('dmitry') ||
                  voice.name.toLowerCase().includes('yuri') ||
                  voice.name.toLowerCase().includes('maxim') ||
                  voice.name.toLowerCase().includes('artem') ||
                  (typeof voice.gender === 'string' && voice.gender.toLowerCase() === 'male') ||
                  (!voices.some(v => v.lang === 'ru-RU' && (v.name.toLowerCase().includes('female') || v.name.toLowerCase().includes('anna') || v.name.toLowerCase().includes('milena'))))
                )
            );
        }

        if (!russianMaleVoice) {
            russianMaleVoice = voices.find(voice => voice.lang === 'ru-RU');
        }

        if (russianMaleVoice) {
            console.log("Выбран русский мужской голос (или доступный русский):", russianMaleVoice.name, russianMaleVoice.lang);
        } else {
            console.warn("Русский мужской голос не найден. Будет использован голос по умолчанию.");
        }
    }

    if (synth.getVoices().length === 0) {
        synth.onvoiceschanged = loadAndSetVoice;
    } else {
        loadAndSetVoice();
    }
}

function speakText(text) {
    if (!synth || !text) return;
    if (synth.speaking) synth.cancel();

    const cleanText = text.replace(/```[\s\S]*?```/g, ' фрагмент кода ')
                          .replace(/`[^`]+`/g, ' код ')
                          .replace(/[\*\_~]/g, '')
                          .replace(/\[(.*?)\]\(.*?\)/g, '$1')
                          .replace(/<[^>]+>/g, ' ')
                          .replace(/\\n/g, '\n')
                          .replace(/\s+/g, ' ')
                          .trim();

    if (!cleanText) return;

    currentUtterance = new SpeechSynthesisUtterance(cleanText);
    currentUtterance.lang = 'ru-RU';

    if (russianMaleVoice) {
        currentUtterance.voice = russianMaleVoice;
        console.log("Используется голос:", russianMaleVoice.name);
    } else {
        console.warn("Русский мужской голос не выбран, используется голос по умолчанию для ru-RU.");
    }

    currentUtterance.onerror = (event) => {
        console.error('[TTS] Ошибка синтеза:', event.error, event.message);
        if (speechError) {
            speechError.textContent = `Ошибка синтеза речи: ${event.error}`;
            speechError.style.display = 'block';
        }
    };
    currentUtterance.onend = () => {
        console.log('[TTS] Синтез речи завершен.');
        currentUtterance = null;
    };

    setTimeout(() => {
        synth.speak(currentUtterance);
    }, 50);
}

function stopSpeech() {
    if (synth && synth.speaking) {
        synth.cancel();
        currentUtterance = null;
    }
}

// --- Функции для чата ---
function addMessage(sender, text, isHistorical = false, attachmentInfo = null) {
    if (!chatBox) return;
    const messageDiv = document.createElement('div');
    messageDiv.classList.add('message', sender.toLowerCase(), 'mb-3', 'p-3', 'rounded-4');

    const senderName = sender === 'User' ? 'Вы' : 'Shaman';
    const badge = document.createElement('strong');
    badge.classList.add('d-block', 'mb-1', 'fw-bold');

    if (sender === 'User') {
        messageDiv.classList.add('bg-body', 'text-end', 'ms-auto', 'border');
        badge.classList.add('text-bg-success', 'px-3', 'rounded-2', 'text-end', 'ms-auto', 'w-50');
        badge.textContent = senderName + ":";
    } else {
        messageDiv.classList.add('bg-body', 'text-start', 'me-auto', 'border');
        badge.classList.add('text-bg-primary', 'px-3', 'rounded-2', 'w-50');
        badge.textContent = senderName + ":";
    }

    const textSpan = document.createElement('div');
    textSpan.innerHTML = text.replace(/\n/g, '<br>');

    messageDiv.appendChild(badge);
    if (attachmentInfo) {
        const attachmentElement = document.createElement('div');
        attachmentElement.classList.add('mt-2', 'attachment-display');
        if (attachmentInfo.type && attachmentInfo.type.startsWith('image/')) {
            const img = document.createElement('img');
            img.src = attachmentInfo.url;
            img.alt = attachmentInfo.name || 'Прикрепленное изображение';
            img.style.maxWidth = '200px';
            img.style.maxHeight = '200px';
            img.classList.add('img-thumbnail');
            attachmentElement.appendChild(img);
        } else {
            const link = document.createElement('a');
            link.href = attachmentInfo.url;
            link.textContent = attachmentInfo.name || 'Прикрепленный файл';
            link.target = '_blank'; // Открывать в новой вкладке
            link.classList.add('bi', 'bi-file-earmark-text', 'me-1'); // Иконка файла
            attachmentElement.appendChild(link);
        }
        messageDiv.appendChild(attachmentElement);
    }
    messageDiv.appendChild(textSpan);
    chatBox.appendChild(messageDiv);

    if (!isHistorical || (chatBox.scrollHeight - chatBox.scrollTop - chatBox.clientHeight < 150) ) {
        chatBox.scrollTop = chatBox.scrollHeight;
    }
}

function setLoading(isLoading) {
    isChatLoading = isLoading;
    if (!loadingIndicator || !sendButton || !userInput || !recordButton || !attachDocButton || !attachImgButton) return;
    loadingIndicator.style.display = isLoading ? 'flex' : 'none';
    sendButton.disabled = isLoading;
    userInput.disabled = isLoading;
    if (recordButton) recordButton.disabled = isLoading;
    if (attachDocButton) attachDocButton.disabled = isLoading;
    if (attachImgButton) attachImgButton.disabled = isLoading;
    userInput.setAttribute('aria-busy', String(isLoading));
    if (!isLoading && userInput) userInput.focus();
}

// --- Логика прикрепления файлов ---
const MAX_FILE_SIZE_BYTES = 10 * 1024 * 1024; // 10MB
const ALLOWED_DOC_TYPES = ['application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'text/plain', 'text/csv'];
const ALLOWED_IMG_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];

function displayAttachmentPreview(file) {
    if (!attachmentPreviewArea) return;
    attachmentPreviewArea.innerHTML = '';
    attachedFile = file;

    const fileInfo = document.createElement('div');
    fileInfo.classList.add('alert', 'alert-info', 'p-2', 'd-flex', 'justify-content-between', 'align-items-center');

    let previewContent;
    if (file.type.startsWith('image/')) {
        previewContent = document.createElement('img');
        previewContent.src = URL.createObjectURL(file);
        previewContent.alt = file.name;
        previewContent.style.maxWidth = '60px';
        previewContent.style.maxHeight = '60px';
        previewContent.classList.add('img-thumbnail', 'me-2');
        previewContent.onload = () => URL.revokeObjectURL(previewContent.src);
    } else {
        previewContent = document.createElement('i');
        previewContent.classList.add('bi', 'bi-file-earmark-text', 'fs-3', 'me-2');
    }

    const fileNameSpan = document.createElement('span');
    fileNameSpan.textContent = file.name + ` (${(file.size / 1024).toFixed(1)} KB)`;
    fileNameSpan.classList.add('text-truncate', 'small');

    const removeBtn = document.createElement('button');
    removeBtn.type = 'button';
    removeBtn.classList.add('btn-close', 'btn-sm');
    removeBtn.setAttribute('aria-label', 'Удалить вложение');
    removeBtn.onclick = () => {
        attachedFile = null;
        attachmentPreviewArea.style.display = 'none';
        attachmentPreviewArea.innerHTML = '';
        if (fileInputDoc) fileInputDoc.value = '';
        if (fileInputImg) fileInputImg.value = '';
    };

    const fileDetails = document.createElement('div');
    fileDetails.classList.add('d-flex', 'align-items-center', 'flex-grow-1', 'overflow-hidden', 'me-2');
    fileDetails.appendChild(previewContent);
    fileDetails.appendChild(fileNameSpan);

    fileInfo.appendChild(fileDetails);
    fileInfo.appendChild(removeBtn);
    attachmentPreviewArea.appendChild(fileInfo);
    attachmentPreviewArea.style.display = 'block';
}

function handleFileSelection(event, allowedTypes, typeName) {
    const file = event.target.files[0];
    if (file) {
        if (file.size > MAX_FILE_SIZE_BYTES) {
            alert(`Файл слишком большой. Максимальный размер: ${MAX_FILE_SIZE_BYTES / (1024 * 1024)}MB`);
            event.target.value = '';
            return;
        }
        if (!allowedTypes.includes(file.type) && !allowedTypes.some(type => file.name.toLowerCase().endsWith(type))) { // Добавим проверку по расширению для .doc/.docx
             let allowedExtensions = allowedTypes.map(t => t.startsWith('.') ? t : '.' + t.split('/')[1]).join(', ');
             if (typeName === 'документов') {
                allowedExtensions = ".pdf, .doc, .docx, .txt, .csv"; // Более понятные расширения
             } else if (typeName === 'изображений') {
                allowedExtensions = ".jpg, .jpeg, .png, .gif, .webp";
             }
            alert(`Недопустимый тип файла для ${typeName}. Разрешены: ${allowedExtensions}. Обнаружен тип: ${file.type || 'неизвестно'}`);
            event.target.value = '';
            return;
        }
        displayAttachmentPreview(file);
    }
}

if (attachDocButton && fileInputDoc) {
    attachDocButton.addEventListener('click', () => fileInputDoc.click());
    fileInputDoc.addEventListener('change', (event) => handleFileSelection(event, ALLOWED_DOC_TYPES.concat(['.doc', '.docx', '.txt', '.csv']), 'документов'));
}

if (attachImgButton && fileInputImg) {
    attachImgButton.addEventListener('click', () => fileInputImg.click());
    fileInputImg.addEventListener('change', (event) => handleFileSelection(event, ALLOWED_IMG_TYPES, 'изображений'));
}

// --- Обновленный обработчик отправки формы ---
if (chatForm) {
    chatForm.addEventListener('submit', async (event) => {
        event.preventDefault();
        const userText = userInput.value.trim();
        if ((!userText && !attachedFile) || userInput.disabled) return;

        if (!currentChatSessionUUID) {
            await handleCreateNewChat();
            if (!currentChatSessionUUID) {
                addMessage('Assistant', "[Ошибка: Не удалось определить или создать сессию чата. Пожалуйста, попробуйте обновить страницу или нажмите 'Начать новый чат'.]");
                setLoading(false);
                return;
            }
        }

        let attachmentDisplayInfo = null;
        if (attachedFile) {
            attachmentDisplayInfo = {
                name: attachedFile.name,
                type: attachedFile.type,
                url: attachedFile.type.startsWith('image/') ? URL.createObjectURL(attachedFile) : '#'
            };
        }

        addMessage('User', userText || '(файл прикреплен)', false, attachmentDisplayInfo);

        const formData = new FormData();
        formData.append('prompt', userText);
        formData.append('chat_session_uuid', currentChatSessionUUID);
        if (attachedFile) {
            formData.append('file', attachedFile, attachedFile.name);
        }

        userInput.value = '';
        accumulatedTranscript = '';
        if (typeof initialTextareaHeight !== 'undefined' && tx) tx.style.height = initialTextareaHeight;
        if (attachedFile) {
            attachedFile = null;
            if(attachmentPreviewArea) {
                attachmentPreviewArea.style.display = 'none';
                attachmentPreviewArea.innerHTML = '';
            }
            if (fileInputDoc) fileInputDoc.value = '';
            if (fileInputImg) fileInputImg.value = '';
        }

        setLoading(true);
        if (speechError) speechError.style.display = 'none';
        stopSpeech();

        try {
            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content');
            const response = await fetch("/api/dialogue_with_file", {
                method: "POST",
                headers: {
                    "Accept": "application/json",
                    "X-CSRF-Token": csrfToken || ''
                },
                body: formData,
            });

            if (!response.ok) {
                let errorText = `Статус ${response.status}`;
                try {
                    const errData = await response.json();
                    errorText = errData.error || errData.message || (typeof errData === 'string' ? errData : errorText);
                } catch (e) { /* Игнорируем, если тело не JSON или пустое */ }
                throw new Error(errorText);
            }

            const data = await response.json();

            if (!data || typeof data.response === 'undefined') {
                throw new Error("Некорректный формат ответа ИИ (поле 'response' отсутствует или пусто)");
            }

            let assistantAttachmentInfo = data.attachment_processed_url ? { url: data.attachment_processed_url, name: "Обработанный файл" } : null;

            addMessage('Assistant', data.response, false, assistantAttachmentInfo);
            if(data.response) speakText(data.response);

            const updatedSession = activeSessionsCache.find(s => s.uuid === currentChatSessionUUID);
            if (updatedSession) {
                updateChatTitle(updatedSession.title);
            }
            await fetchAndPopulateSessionHistory();
            setActiveSessionLink(currentChatSessionUUID);

        } catch (error) {
            console.error("Ошибка при отправке/получении ответа от ИИ (с файлом):", error);
            addMessage('Assistant', `[Ошибка: ${error.message || 'Не удалось получить ответ от ИИ.'}]`);
        } finally {
            setLoading(false);
        }
    });
}

// --- Функции для работы с API сессий ---
async function fetchAndPopulateSessionHistory() {
    if (!sessionHistoryList) return null;
    sessionHistoryList.innerHTML = '<li class="nav-item"><a href="#" class="nav-link text-secondary disabled"><div class="spinner-border spinner-border-sm" role="status"></div> Загрузка...</a></li>';
    try {
        const response = await fetch('/api/chat_sessions');
        if (!response.ok) throw new Error(`Ошибка ${response.status}`);
        activeSessionsCache = await response.json();
        sessionHistoryList.innerHTML = '';
        if (activeSessionsCache && activeSessionsCache.length > 0) {
            activeSessionsCache.forEach(session => {
                const li = document.createElement('li'); li.classList.add('nav-item');
                const a = document.createElement('a');
                a.href = '#'; a.classList.add('nav-link', 'text-truncate', 'py-1', 'px-2', 'mb-1');
                a.style.fontSize = '0.875rem';
                a.textContent = session.title || `Диалог от ${new Date(session.updated_at).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })}`;
                a.setAttribute('data-session-uuid', session.uuid);
                a.setAttribute('title', a.textContent);
                li.appendChild(a);
                sessionHistoryList.appendChild(li);
            });
            return activeSessionsCache[0];
        } else {
            sessionHistoryList.innerHTML = '<li class="nav-item"><span class="nav-link text-muted px-2 py-1" style="font-size: 0.875rem;">Нет прошлых диалогов</span></li>';
            return null;
        }
    } catch (error) {
        console.error("Не удалось загрузить список сессий:", error);
        sessionHistoryList.innerHTML = '<li class="nav-item"><span class="nav-link text-danger px-2 py-1" style="font-size: 0.875rem;">Ошибка загрузки</span></li>';
        return null;
    }
}

async function loadMessagesForSession(sessionUUID, sessionTitle = "Диалог") {
    if (!chatBox || !sessionUUID || isChatLoading) return;
    setLoading(true);
    chatBox.innerHTML = '';
    currentChatSessionUUID = sessionUUID;
    console.log(`Загрузка сообщений для сессии: ${sessionUUID} (${sessionTitle})`);
    setActiveSessionLink(sessionUUID);
    updateChatTitle(sessionTitle);

    try {
        const response = await fetch(`/api/chat_session_messages?uuid=${encodeURIComponent(sessionUUID)}`);
        if (!response.ok) throw new Error(`Ошибка ${response.status}`);
        const messages = await response.json();
        if (messages && messages.length > 0) {
            messages.forEach(msg => addMessage(msg.Role === 'user' ? 'User' : 'Assistant', msg.Content, true));
        } else {
            // Если сессия новая и пустая, можно добавить приветственное сообщение или ничего не делать
             addMessage('Assistant', `Это начало вашего диалога "${sessionTitle}". Чем могу помочь?`, true);
        }
        chatBox.scrollTop = chatBox.scrollHeight;
    } catch (error) {
        console.error("Ошибка загрузки сообщений сессии:", error);
        addMessage('Assistant', `[Ошибка: Не удалось загрузить сообщения сессии.]`, true);
    }
    finally { setLoading(false); if (userInput) userInput.focus(); }
}

async function handleCreateNewChat() {
    if (isChatLoading) return;
    setLoading(true);
    console.log("Создание новой сессии чата...");
    try {
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content');
        const response = await fetch('/api/chat_session_create', {
            method: 'POST',
            headers: { "Content-Type": "application/json", "Accept": "application/json", "X-CSRF-Token": csrfToken || '' },
            body: JSON.stringify({ title: "" })
        });
        if (!response.ok) throw new Error(`Ошибка ${response.status}`);
        const newSession = await response.json();

        if (newSession && newSession.uuid) {
            currentChatSessionUUID = newSession.uuid;
            chatBox.innerHTML = ''; 
            addMessage('Assistant', `${newSession.title || 'Новый диалог'}. Чем могу помочь?`, true); // Приветственное сообщение
            await fetchAndPopulateSessionHistory();
            setActiveSessionLink(currentChatSessionUUID);
            updateChatTitle(newSession.title || "Новый диалог");
        } else { throw new Error("Ответ сервера не содержит UUID новой сессии."); }
    } catch (error) {
        console.error("Ошибка при создании новой сессии:", error);
        addMessage('Assistant', `[Ошибка: Не удалось создать новую сессию. ${error.message}]`);
    }
    finally { setLoading(false); if (userInput) userInput.focus(); }
}

function setActiveSessionLink(sessionUUID) {
    if (!sessionHistoryList) return;
    sessionHistoryList.querySelectorAll('a.nav-link').forEach(link => {
        if (link.getAttribute('data-session-uuid') === sessionUUID) {
            link.classList.add('active', 'bg-secondary', 'text-white'); // Добавляем text-white для лучшей читаемости на темном фоне
            link.classList.remove('text-link'); // text-link может быть вашим классом для неактивных ссылок
        } else {
            link.classList.remove('active', 'bg-secondary', 'text-white');
            link.classList.add('text-link'); // Или ваш стандартный класс для ссылок сайдбара
        }
    });
}

function updateChatTitle(title) {
    if (currentChatTitleElement) {
        currentChatTitleElement.textContent = title || "Диалог";
    } else {
        console.warn("Элемент currentChatTitleElement не найден");
    }
}

// --- Обработка кликов в сайдбаре ---
if (sessionHistoryList) {
    sessionHistoryList.addEventListener('click', (event) => {
        const targetLink = event.target.closest('a.nav-link');
        if (!targetLink || targetLink.classList.contains('disabled')) return;
        event.preventDefault();
        const sessionUUID = targetLink.getAttribute('data-session-uuid');

        if (!sessionUUID || (sessionUUID === currentChatSessionUUID && chatBox.children.length > 1 && !isChatLoading) ) {
             if (sessionUUID === currentChatSessionUUID && chatBox.children.length <= 1) {
                 // Если кликнули на текущую пустую сессию, ничего не делаем или фокусируемся на вводе
                 if (userInput) userInput.focus();
             }
             return; // Не перезагружаем, если сессия та же и не пустая, или идет загрузка
        }

        const sessionData = activeSessionsCache.find(s => s.uuid === sessionUUID);
        loadMessagesForSession(sessionUUID, sessionData ? sessionData.title : "Загрузка...");

        const offcanvasElement = document.getElementById('chatGptSidebar');
        if (offcanvasElement && offcanvasElement.classList.contains('show')) {
            try {
                const bsOffcanvas = bootstrap.Offcanvas.getInstance(offcanvasElement);
                if (bsOffcanvas) {
                    bsOffcanvas.hide();
                }
            }
            catch (e) { console.error("Ошибка скрытия Offcanvas:", e); }
        }
    });
}

if (newChatButton) {
    newChatButton.addEventListener('click', (event) => {
        event.preventDefault();
        if (isChatLoading) return;

        if (chatBox && currentChatSessionUUID && chatBox.children.length <= 1 && !userInput.value && !attachedFile) {
             console.log("Текущий чат уже новый или пуст. Фокус на поле ввода.");
             if (userInput) userInput.focus();
             return;
        }
        handleCreateNewChat();
    });
}

// --- Автовысота Textarea ---
const tx = document.getElementById('user-input');
let initialTextareaHeight;
if (tx) {
  initialTextareaHeight = (tx.scrollHeight < 36 ? 36 : tx.scrollHeight) + "px";

  window.OnInput = function() {
    this.style.height = 0;
    this.style.height = (this.scrollHeight) + "px";
  }
  tx.addEventListener("input", OnInput, false);

  tx.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        if (chatForm && !userInput.disabled) {
            if (!currentChatSessionUUID && (userInput.value.trim() || attachedFile)) { 
                 handleCreateNewChat().then(() => { 
                    if (currentChatSessionUUID) {
                        chatForm.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true})); 
                    } else {
                        console.error("Не удалось создать сессию перед отправкой сообщения.");
                        addMessage('Assistant', "[Ошибка: Не удалось создать сессию. Попробуйте еще раз.]");
                    }
                 });
            } else if (currentChatSessionUUID && (userInput.value.trim() || attachedFile)) {
                 chatForm.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
            }
        }
    }
  });

   if (tx.value) { 
    OnInput.call(tx);
   }
}

// --- Инициализация ---
document.addEventListener('DOMContentLoaded', async () => {
    if (window.location.pathname === '/dashboard') {
        document.body.classList.add('dashboard-page-active');
        const latestSession = await fetchAndPopulateSessionHistory();
        if (latestSession && latestSession.uuid) {
            await loadMessagesForSession(latestSession.uuid, latestSession.title);
        } else {
            await handleCreateNewChat(); 
        }
    }
    if (userInput) userInput.focus();
});

console.log("Логика чата инициализирована."); 