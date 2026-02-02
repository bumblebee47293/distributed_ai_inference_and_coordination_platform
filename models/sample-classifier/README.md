# Sample Classifier Model

This directory contains a sample ResNet18 image classifier exported to ONNX format for use with Triton Inference Server.

## Setup

1. **Install dependencies:**

   ```bash
   pip install -r requirements.txt
   ```

2. **Export the model:**

   ```bash
   python export_model.py
   ```

3. **Organize for Triton:**

   ```bash
   mkdir -p resnet18/1
   mv resnet18.onnx resnet18/1/model.onnx
   cp config.pbtxt resnet18/
   ```

4. **Model repository structure:**
   ```
   models/
   └── resnet18/
       ├── config.pbtxt
       └── 1/
           └── model.onnx
   ```

## Model Details

- **Architecture:** ResNet18
- **Framework:** PyTorch → ONNX
- **Input:** `[batch_size, 3, 224, 224]` (RGB images)
- **Output:** `[batch_size, 1000]` (ImageNet classes)
- **Batch size:** Dynamic (max 8)

## Testing

Test the model locally:

```python
import onnxruntime as ort
import numpy as np

session = ort.InferenceSession("resnet18/1/model.onnx")
input_data = np.random.randn(1, 3, 224, 224).astype(np.float32)
result = session.run(None, {"input": input_data})
print(f"Prediction: {np.argmax(result[0])}")
```

## Triton Deployment

The model is configured for Triton with:

- **Dynamic batching** (preferred batch sizes: 4, 8)
- **2 CPU instances** for parallel inference
- **Max queue delay:** 100 microseconds

## Adding More Models

To add additional models:

1. Create a new directory under `models/`
2. Export your model to ONNX
3. Create a `config.pbtxt` file
4. Organize in Triton format: `model_name/version/model.onnx`
5. Register in the metadata service
