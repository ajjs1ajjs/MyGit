import pytest
from django.contrib.auth import get_user_model
from django.db import IntegrityError

User = get_user_model()


@pytest.mark.django_db
class TestUserModel:
    def test_create_user(self):
        user = User.objects.create_user(
            email="test@example.com",
            username="testuser",
            password="securepass123",
        )
        assert user.email == "test@example.com"
        assert user.username == "testuser"
        assert user.check_password("securepass123")
        assert user.is_active is True
        assert user.is_superuser is False

    def test_create_superuser(self):
        admin = User.objects.create_superuser(
            email="root@example.com",
            username="root",
            password="adminpass123",
        )
        assert admin.is_superuser is True
        assert admin.is_staff is True

    def test_unique_email(self):
        User.objects.create_user(email="dup@example.com", username="user1", password="pass1")
        with pytest.raises(IntegrityError):
            User.objects.create_user(email="dup@example.com", username="user2", password="pass2")

    def test_user_str(self):
        user = User.objects.create_user(
            email="str@example.com", username="struser", password="pass"
        )
        assert str(user) == "struser"
