// static/js/chat.js (или новый static/js/legal.js)
document.addEventListener('DOMContentLoaded', () => {
    const termsModal = document.getElementById('termsModal');
    const privacyModal = document.getElementById('privacyPolicyModal');

    const loadModalContent = async (modalElement, modalBodyId, docType) => {
        if (!modalElement) return;

        const modalBody = document.getElementById(modalBodyId);
        if (modalBody && modalBody.dataset.loaded !== 'true') {
            modalBody.innerHTML = '<p>Загрузка...</p>';
        }
        
        try {
            const response = await fetch(`/api/legal/${docType}`);
            if (!response.ok) {
                throw new Error(`Ошибка HTTP: ${response.status}`);
            }
            const data = await response.json();
            
            if (modalBody) {
                const modalTitleElement = modalElement.querySelector('.modal-title');
                if (modalTitleElement && data.title) {
                    modalTitleElement.textContent = data.title;
                }
                modalBody.innerHTML = data.Content; 
                modalBody.dataset.loaded = 'true';
            }
        } catch (error) {
            console.error(`Не удалось загрузить ${docType}:`, error);
            if (modalBody) {
                modalBody.innerHTML = `<p class="text-danger">Не удалось загрузить документ. Пожалуйста, попробуйте позже.</p>`;
            }
        }
    };

    if (termsModal) {
        termsModal.addEventListener('show.bs.modal', function () {
            loadModalContent(termsModal, 'termsModalBody', 'terms');
        });
    }

    if (privacyModal) {
        privacyPolicyModal.addEventListener('show.bs.modal', function () {
            loadModalContent(privacyModal, 'privacyPolicyModalBody', 'privacy');
        });
    }

    window.closeCurrentAndOpenNewModal = function(currentModalSelector, newModalSelector) {
        const currentModalElement = document.querySelector(currentModalSelector);
        const newModalElement = document.querySelector(newModalSelector);

        if (currentModalElement) {
            const currentBsModal = bootstrap.Modal.getInstance(currentModalElement);
            if (currentBsModal) {
                currentBsModal.hide();
                currentModalElement.addEventListener('hidden.bs.modal', function () {
                    if (newModalElement) {
                        const newBsModal = bootstrap.Modal.getInstance(newModalElement) || new bootstrap.Modal(newModalElement);
                        newBsModal.show();
                    }
                }, { once: true }); 
                return;
            }
        }
        if (newModalElement) {
            const newBsModal = bootstrap.Modal.getInstance(newModalElement) || new bootstrap.Modal(newModalElement);
            newBsModal.show();
        }
    };
});