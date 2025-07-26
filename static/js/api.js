// static/js/api.js
export async function sendPromptToAPI(prompt) {
    console.log(`Отправка промпта на /api/dialogue: "${prompt}"`);
    try {
        // Читаем CSRF-токен из мета-тега
        const csrfTokenElement = document.querySelector('meta[name="csrf-token"]');
        const csrfToken = csrfTokenElement ? csrfTokenElement.getAttribute('content') : '';

        if (!csrfToken) {
            console.error("CSRF токен не найден на странице!");
            // Можно выбросить ошибку или обработать иначе
        }

        const response = await fetch("/api/dialogue", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Accept": "application/json",
                "X-CSRF-Token": csrfToken // <-- ДОБАВЛЕН CSRF ТОКЕН В ЗАГОЛОВОК
            },
            body: JSON.stringify({ prompt: prompt }),
        });

        if (!response.ok) {
            let errorText = `Статус ${response.status}: ${response.statusText}`;
            try {
                const errorBody = await response.json();
                errorText = errorBody.message || errorBody.error || JSON.stringify(errorBody) || errorText;
            } catch (e) { /* игнорируем */ }
            throw new Error(`Ошибка сервера: ${errorText}`);
        }

        const data = await response.json();

        if (!data || typeof data.response === 'undefined') {
            throw new Error("Некорректный формат ответа от сервера: отсутствует поле 'response'");
        }

        console.log(`Получен ответ от ИИ: "${data.response}"`);
        return data.response;

    } catch (error) {
        console.error("Ошибка при вызове API /api/dialogue:", error);
        throw error;
    }
}