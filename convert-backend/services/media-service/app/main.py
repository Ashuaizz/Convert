from fastapi import FastAPI

app = FastAPI(title="Convert Media Service", version="0.1.0")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok", "service": "media-service"}


@app.post("/process")
def process() -> dict[str, str]:
    return {
        "status": "accepted",
        "message": "media processing is not implemented yet",
    }
