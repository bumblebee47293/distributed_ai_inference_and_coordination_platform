"""
Export a sample PyTorch model to ONNX format for Triton Inference Server.
This creates a simple ResNet18 classifier for demonstration purposes.
"""

import torch
import torchvision.models as models
import numpy as np

def export_resnet18():
    """Export ResNet18 model to ONNX format."""
    
    # Load pre-trained ResNet18
    model = models.resnet18(pretrained=True)
    model.eval()
    
    # Create dummy input (batch_size=1, channels=3, height=224, width=224)
    dummy_input = torch.randn(1, 3, 224, 224)
    
    # Export to ONNX
    output_path = "resnet18.onnx"
    torch.onnx.export(
        model,
        dummy_input,
        output_path,
        export_params=True,
        opset_version=14,
        do_constant_folding=True,
        input_names=['input'],
        output_names=['output'],
        dynamic_axes={
            'input': {0: 'batch_size'},
            'output': {0: 'batch_size'}
        }
    )
    
    print(f"Model exported to {output_path}")
    print(f"Input shape: {dummy_input.shape}")
    print(f"Model parameters: {sum(p.numel() for p in model.parameters()):,}")
    
    return output_path

def test_onnx_model(model_path):
    """Test the exported ONNX model."""
    import onnxruntime as ort
    
    # Create inference session
    session = ort.InferenceSession(model_path)
    
    # Get input/output names
    input_name = session.get_inputs()[0].name
    output_name = session.get_outputs()[0].name
    
    print(f"\nModel input: {input_name}")
    print(f"Model output: {output_name}")
    
    # Create test input
    test_input = np.random.randn(1, 3, 224, 224).astype(np.float32)
    
    # Run inference
    result = session.run([output_name], {input_name: test_input})
    
    print(f"\nTest inference successful!")
    print(f"Output shape: {result[0].shape}")
    print(f"Top 5 predictions: {np.argsort(result[0][0])[-5:][::-1]}")

if __name__ == "__main__":
    print("Exporting ResNet18 model to ONNX...")
    model_path = export_resnet18()
    
    print("\nTesting ONNX model...")
    test_onnx_model(model_path)
    
    print("\n✅ Model export and validation complete!")
    print(f"Move {model_path} to models/resnet18/1/ for Triton deployment")
