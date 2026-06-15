import os
import tempfile
from dataclasses import dataclass
from pathlib import PurePosixPath
from typing import Literal
from urllib.parse import urlparse

import boto3
from botocore.config import Config
from fastapi import FastAPI, HTTPException
from PIL import Image
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
    status: Literal["succeeded"]
    job_id: str
    message: str
    output: FileRef


@dataclass(frozen=True)
class ObjectRef:
    bucket: str
    key: str


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok", "service": "image-service"}


@app.post("/process/image.resize", response_model=ImageResizeResponse)
def resize_image(request: ImageResizeRequest) -> ImageResizeResponse:
    input_ref = parse_s3_uri(request.input.uri)
    output_uri = request.output_uri or default_output_uri(request)
    output_ref = parse_s3_uri(output_uri)

    try:
        output_size = process_resize(request, input_ref, output_ref)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"image resize failed: {exc}") from exc

    content_type = content_type_for_format(request.options.format)
    return ImageResizeResponse(
        status="succeeded",
        job_id=request.job_id,
        message="image resized",
        output=FileRef(
            file_id=f"{request.input.file_id}_resized",
            uri=output_uri,
            content_type=content_type,
            size=output_size,
        ),
    )


def process_resize(request: ImageResizeRequest, input_ref: ObjectRef, output_ref: ObjectRef) -> int:
    s3 = s3_client()
    output_format = pillow_format(request.options.format)
    content_type = content_type_for_format(request.options.format)

    with tempfile.TemporaryDirectory(prefix="convert-image-") as temp_dir:
        input_path = os.path.join(temp_dir, "input")
        output_path = os.path.join(temp_dir, f"output.{normalized_extension(request.options.format)}")

        s3.download_file(input_ref.bucket, input_ref.key, input_path)

        with Image.open(input_path) as image:
            if image.width * image.height > 100_000_000:
                raise ValueError("image pixel count exceeds 100MP limit")
            resized = image.resize((request.options.width, request.options.height))
            if output_format == "JPEG" and resized.mode not in ("RGB", "L"):
                resized = resized.convert("RGB")
            save_kwargs = {}
            if output_format in {"JPEG", "WEBP"}:
                save_kwargs["quality"] = request.options.quality
            resized.save(output_path, output_format, **save_kwargs)

        s3.upload_file(
            output_path,
            output_ref.bucket,
            output_ref.key,
            ExtraArgs={"ContentType": content_type},
        )
        return os.path.getsize(output_path)


def s3_client():
    endpoint = os.getenv("CONVERT_STORAGE_ENDPOINT", "http://localhost:9000")
    region = os.getenv("CONVERT_STORAGE_REGION", "us-east-1")
    access_key = os.getenv("CONVERT_STORAGE_ACCESS_KEY_ID", "convert")
    secret_key = os.getenv("CONVERT_STORAGE_SECRET_ACCESS_KEY", "convert-secret")
    force_path_style = os.getenv("CONVERT_STORAGE_FORCE_PATH_STYLE", "true").lower() == "true"
    return boto3.client(
        "s3",
        endpoint_url=endpoint,
        region_name=region,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        config=Config(s3={"addressing_style": "path" if force_path_style else "auto"}),
    )


def parse_s3_uri(uri: str) -> ObjectRef:
    parsed = urlparse(uri)
    if parsed.scheme != "s3" or not parsed.netloc or not parsed.path.strip("/"):
        raise ValueError("uri must be a valid s3://bucket/key reference")
    return ObjectRef(bucket=parsed.netloc, key=parsed.path.lstrip("/"))


def default_output_uri(request: ImageResizeRequest) -> str:
    input_ref = parse_s3_uri(request.input.uri)
    input_name = PurePosixPath(input_ref.key).stem or request.input.file_id
    extension = normalized_extension(request.options.format)
    output_key = f"results/{request.job_id}/{input_name}.{extension}"
    return f"s3://{input_ref.bucket}/{output_key}"


def pillow_format(format_name: str) -> str:
    if format_name in {"jpg", "jpeg"}:
        return "JPEG"
    if format_name == "png":
        return "PNG"
    if format_name == "webp":
        return "WEBP"
    raise ValueError(f"unsupported image format {format_name}")


def normalized_extension(format_name: str) -> str:
    return "jpg" if format_name == "jpeg" else format_name


def content_type_for_format(format_name: str) -> str:
    if format_name in {"jpg", "jpeg"}:
        return "image/jpeg"
    return f"image/{format_name}"
