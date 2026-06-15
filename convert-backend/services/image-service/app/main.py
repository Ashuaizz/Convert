from fastapi import FastAPI

app = FastAPI(title="Convert Image Service", version="0.1.0")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok", "service": "image-service"}


@app.post("/process")
def process() -> dict[str, str]:
    return {
        "status": "accepted",
        "message": "image processing is not implemented yet",
    }
