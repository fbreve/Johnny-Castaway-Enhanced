package main

import rl "github.com/gen2brain/raylib-go/raylib"

// hdrDiagnosticLayers exercises multiple offscreen passes and all five real
// game fragment shaders without loading proprietary assets or story state.
type hdrDiagnosticLayers struct {
	layers []rl.RenderTexture2D
	final rl.RenderTexture2D
	shaders []rl.Shader
	frame int
}

func newHDRDiagnosticLayers() *hdrDiagnosticLayers {
	h := &hdrDiagnosticLayers{}
	for i := 0; i < 4; i++ {
		h.layers = append(h.layers, rl.LoadRenderTexture(screenWidth, screenHeight))
	}
	h.final = rl.LoadRenderTexture(screenWidth, screenHeight)
	h.shaders = []rl.Shader{
		rl.LoadShaderFromMemory("", sharpBilinearFs),
		rl.LoadShaderFromMemory("", ditherBlendFs),
		rl.LoadShaderFromMemory("", smartDitherFs),
		rl.LoadShaderFromMemory("", scanlineFs),
		rl.LoadShaderFromMemory("", crtSimulatorFs),
	}
	return h
}

func (h *hdrDiagnosticLayers) setUniforms(shader rl.Shader) {
	textureSize := []float32{screenWidth, screenHeight}
	renderSize := []float32{screenWidth, screenHeight}
	for _, name := range []string{"textureSize", "renderSize"} {
		loc := rl.GetShaderLocation(shader, name)
		if loc >= 0 {
			if name == "textureSize" {
				rl.SetShaderValue(shader, loc, textureSize, rl.ShaderUniformVec2)
			} else {
				rl.SetShaderValue(shader, loc, renderSize, rl.ShaderUniformVec2)
			}
		}
	}
	for _, entry := range []struct { name string; value float32 }{
		{"scanlineWeight", 0.15}, {"curvatureWeight", 0.0}, {"vignetteWeight", 0.0},
	} {
		loc := rl.GetShaderLocation(shader, entry.name)
		if loc >= 0 {
			rl.SetShaderValue(shader, loc, []float32{entry.value}, rl.ShaderUniformFloat)
		}
	}
}

func (h *hdrDiagnosticLayers) composeFrame() {
	h.frame++
	for i, layer := range h.layers {
		rl.BeginTextureMode(layer)
		rl.ClearBackground(rl.Blank)
		rl.DrawRectangle(int32(20+i*35), int32(20+i*30), 230, 180,
			rl.NewColor(uint8(45+i*30), uint8(70+i*20), uint8(110+i*20), 170))
		rl.EndTextureMode()
	}
	rl.BeginTextureMode(h.final)
	rl.ClearBackground(rl.Black)
	for i, layer := range h.layers {
		shader := h.shaders[i]
		h.setUniforms(shader)
		rl.BeginShaderMode(shader)
		rl.DrawTexturePro(layer.Texture,
			rl.NewRectangle(0, 0, screenWidth, -screenHeight),
			rl.NewRectangle(0, 0, screenWidth, screenHeight),
			rl.NewVector2(0, 0), 0, rl.White)
		rl.EndShaderMode()
	}
	rl.EndTextureMode()
}

func (h *hdrDiagnosticLayers) present() {
	shader := h.shaders[4]
	h.setUniforms(shader)
	rl.BeginShaderMode(shader)
	rl.DrawTexturePro(h.final.Texture,
		rl.NewRectangle(0, 0, screenWidth, -screenHeight),
		rl.NewRectangle(0, 0, float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())),
		rl.NewVector2(0, 0), 0, rl.White)
	rl.EndShaderMode()
}

// Match graphicsEnd's relevant order: release render targets before shaders.
func (h *hdrDiagnosticLayers) close() {
	rl.UnloadRenderTexture(h.final)
	for i := len(h.layers)-1; i >= 0; i-- {
		rl.UnloadRenderTexture(h.layers[i])
	}
	for _, shader := range h.shaders {
		if shader.ID > 0 {
			rl.UnloadShader(shader)
		}
	}
}
