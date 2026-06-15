from typing import Literal

from fastapi import FastAPI
from pydantic import BaseModel, Field, field_validator

app = FastAPI(title="Convert Image Service", version="0.1.0")


class FileRef(BaseModel):
    file_id: str = Field(min_length=1)
    uri: str = Field(min_length=1)
    content_type: str = Field(min_length=1)
    size: int = Field(ge=0)


class ResizeOptions(BaseModel):
    width: int = Field(ge=1, le=10000)
    height: int = Field(ge=1, le=10000)
    format: Literal["jpg", "jpeg", "png", "webp"] = "webp"
    quality: int = Field(default=85, ge=1, le=100)


class ImageResizeRequest(BaseModel):
    request_id: str | None = None
    job_id: str = Field(min_length=1)
    input: FileRef
    output_uri: str | None = None
    options: ResizeOptions

    @field_validator("output_uri")
    @classmethod
    def validate_output_uri(cls, value: str | None) -> str | None:
        if value is not None and not value.startswith("s3://"):
            raise ValueError("output_uri must use s3:// scheme")
        return value


class ImageResizeResponse(BaseModel):
    status: Literal["accepted"]
    job_id: str
    message: str
    output: FileRef | None = None


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok", "service": "image-service"}


@app.post("/process/image.resize", response_model=ImageResizeResponse)
def resize_image(request: ImageResizeRequest) -> ImageResizeResponse:
    return ImageResizeResponse(
        status="accepted",
        job_id=request.job_id,
        message="image.resize contract accepted; processing is not implemented yet",
    )
