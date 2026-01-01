// Client-side validation for password reset form
document.addEventListener('DOMContentLoaded', function() {
    const form = document.querySelector('form');
    if (!form) return;

    form.addEventListener('submit', function(event) {
        const newPassword = document.getElementById('new_password').value;
        const confirmPassword = document.getElementById('confirm_password').value;
        
        if (newPassword !== confirmPassword) {
            event.preventDefault();
            alert('New passwords do not match!');
            return false;
        }
        
        if (newPassword.length < 8) {
            event.preventDefault();
            alert('Password must be at least 8 characters long!');
            return false;
        }
        
        return true;
    });
});
