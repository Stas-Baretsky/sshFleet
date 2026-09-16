package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "SSH Fleet",
		Width:     1180,
		Height:    780,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: options.NewRGB(247, 247, 249),
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon: appIcon,

			//
			// Совпадает со StartupWMClass в .desktop-файле,
			// чтобы окно правильно матчилось с ярлыком приложения
			//
			ProgramName: "sshfleet",
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
