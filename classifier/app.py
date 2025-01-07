from flask import Flask, request, jsonify
from flask_cors import CORS
from transformers import CLIPProcessor, CLIPModel
import torch
import requests
from PIL import Image
from io import BytesIO
import os
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)
log = logging.getLogger('werkzeug')
log.setLevel(logging.ERROR)

app = Flask(__name__)
CORS(app)

# startup
try:
    # Load CLIP model and processor
    model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32")
    processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")
    device = "mps" if torch.mps.is_available() else "cpu"
    model = model.to(device)
except Exception as e:
    logger.error(f"Failed to load model: {e}")
    raise

def process_image_url(url):
    """Download and process image from URL"""
    try:
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        img = Image.open(BytesIO(response.content))
        img = img.convert('RGB')  # Ensure RGB format
        return img
    except requests.exceptions.RequestException as e:
        logger.error(f"Failed to download image: {e}")
        raise
    except Exception as e:
        logger.error(f"Failed to process image: {e}")
        raise

def classify_bird(image):
    """Classify if image contains a bird using CLIP."""
    inputs = processor(
        images=image,
        text=["a photo containing a bird", "a photo not containing a bird"],
        return_tensors="pt",
        padding=True
    ).to(device)

    with torch.no_grad():
        outputs = model(**inputs)
        logits_per_image = outputs.logits_per_image
        probs = torch.nn.functional.softmax(logits_per_image, dim=1)

        is_bird = probs[0][0].item() > probs[0][1].item()
        confidence = float(max(probs[0]).item())

        return is_bird, confidence

@app.route('/potentialBird', methods=['POST'])
def classify_image():
    try:
        data = request.get_json()

        if not data or 'image_url' not in data:
            logger.error("No image URL provided in request")
            return jsonify({'error': 'No URL provided'}), 400

        img = process_image_url(data['image_url'])
        is_bird, confidence = classify_bird(img)
        if is_bird:
            logger.info(f"Image {data['image_url']} contains a bird with confidence: {confidence}")

        return jsonify({
            'label': 'bird' if is_bird else 'not_bird',
            'confidence': confidence
        })

    except Exception as e:
        logger.error(f"Error during classification: {str(e)}", exc_info=True)
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    port = int(os.getenv('PORT', 12000))
    app.run(host='0.0.0.0', port=port, debug=False)