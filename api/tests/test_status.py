from fastapi.testclient import TestClient
from main import app

client = TestClient(app)

def test_status():
    response = client.get("/status")
    assert response.status_code == 200
    response = response.json()
    assert "project" in response
    assert "version" in response
