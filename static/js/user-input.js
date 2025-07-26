// В static/js/user-input.js 
const tx = document.getElementById('user-input');
if (tx) {
  const initialHeight = tx.scrollHeight + 'px'; // Запомним начальную высоту
  tx.style.height = initialHeight; // Установим начальную высоту

  tx.addEventListener("input", OnInput, false);

  function OnInput() {
    this.style.height = 0; // Сбросить высоту до минимальной
    this.style.height = (this.scrollHeight) + "px"; // Установить новую высоту по контенту
  }

  // Сбрасывать высоту после отправки формы
  const chatForm = document.getElementById('chat-form');
  if (chatForm) {
      chatForm.addEventListener('submit', () => {
          setTimeout(() => { // Небольшая задержка, чтобы значение успело очиститься
               tx.style.height = initialHeight;
          }, 0);
      });
  }
}