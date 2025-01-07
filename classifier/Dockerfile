FROM python:3.12
WORKDIR /app
COPY requirements.txt requirements.txt
RUN pip install --no-cache-dir -r requirements.txt
# Pre-download the model during build to reduce container startup time
RUN python -c "from transformers import ViTForImageClassification, ViTFeatureExtractor; model = ViTForImageClassification.from_pretrained('medhir/vit-base-patch16-224-in21k-selfie'); feature_extractor = ViTFeatureExtractor.from_pretrained('medhir/vit-base-patch16-224-in21k-selfie')"
COPY . .
ENV PORT=12000
ENTRYPOINT ["python", "app.py"]