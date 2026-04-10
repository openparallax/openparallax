# Image & Video Generation

OpenParallax can generate and edit images and generate videos using AI providers.

## Configuration

Image and video generation use the `roles.image` and `roles.video` model pool mappings:

```yaml
models:
  - name: dalle
    provider: openai
    model: gpt-image-1
    api_key_env: OPENAI_API_KEY
  - name: sora
    provider: openai
    model: sora-2
    api_key_env: OPENAI_API_KEY

roles:
  image: dalle
  video: sora
```

If no `image` or `video` role is configured, the corresponding tool group is not loaded.

## Supported Providers

### Image Generation

| Provider | Model | Features |
|----------|-------|----------|
| OpenAI | `gpt-image-1` (DALL-E) | Generate |
| Google | `imagen-3.0-generate-002` (Imagen) | Generate |
| Stability | StabilityAI API | Generate |

### Video Generation

| Provider | Model | Features |
|----------|-------|----------|
| OpenAI | `sora-2` (Sora) | Generate |

## Tools

| Tool | Action Type | Description |
|------|-------------|-------------|
| `generate_image` | `ActionGenerateImage` | Generate an image from a text prompt |
| `edit_image` | `ActionEditImage` | Planned — not yet implemented for any provider |
| `generate_video` | `ActionGenerateVideo` | Generate a video from a text prompt |

Generated files are saved to the workspace and displayed as artifacts in the web UI canvas panel.

::: info Planned
Image editing (`edit_image`) is defined in the tool schema but not yet implemented for any provider. Calling it returns an error explaining the limitation.
:::

## Usage

The agent uses these tools automatically when the user asks for visual content:

- "Generate an image of a sunset over the ocean"
- "Edit this screenshot to remove the sidebar"
- "Create a 5-second video of a spinning logo"

All generation requests go through the Shield pipeline. The image/video provider, prompt, and parameters are evaluated before execution.
