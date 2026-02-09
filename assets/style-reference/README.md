# Style Reference Images

Drop your style reference image in this folder.

The image defines the artistic style (brushwork, lighting, color palette) that Gemini will apply when generating clip images. The model copies **only the style**—not the subject or scene.

**Filename:** Name your file (e.g., `sample.jpeg`, `luminous-regal.jpeg`).

**Env variable:** Set in your `.env`:

```
GEMINI_STYLE_REFERENCE_IMAGE=assets/style-reference/sample.jpeg
```

For Docker: The path is resolved from the project root. Default: `assets/style-reference/sample.jpeg`.
