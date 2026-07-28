import logging

import numpy as np
import cv2
from fastapi import FastAPI, File, UploadFile, HTTPException
from ultralytics import YOLO

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("inference")

app = FastAPI(title="Inference Service")

model = YOLO("yolov8n.pt")

@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/infer")
async def infer(file: UploadFile = File(...)):
    if not file.content_type or not file.content_type.startswith("image/"):
        raise HTTPException(status_code=400, detail="Sadece görüntü dosyaları kabul edilir")

    contents = await file.read()
    nparr = np.frombuffer(contents, np.uint8)
    img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)

    if img is None:
        raise HTTPException(status_code=400, detail="Görüntü decode edilemedi")

    results = model(img, verbose=False)

    detections = []
    for r in results:
        for box in r.boxes:
            detections.append({
                "class": model.names[int(box.cls)],
                "confidence": round(float(box.conf), 4),
                "bbox": [round(x, 2) for x in box.xyxy[0].tolist()],  # [x1, y1, x2, y2]
            })

    return {
        "filename": file.filename,
        "detection_count": len(detections),
        "detections": detections,
    }
