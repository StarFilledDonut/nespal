package main

import (
	"embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed palettes/**.pal
var palettes embed.FS

// TODO: Detect wrong types of .pal

// Extracts the color palette from an NES/FAMICOM pal file
// will not work with pal files for other uses
func load_palette(path string) (color.Palette, error) {
	file, err := os.Open(path)
	if err != nil {
		file.Close()
		return nil, err
	}
	defer file.Close()
	const PALETTE_SIZE = 64

	// NES color palette has 64 colors in RGB format
	data := make([]byte, PALETTE_SIZE*3)
	_, err = io.ReadFull(file, data)
	if err != nil {
		return nil, err
	}

	palette := make(color.Palette, PALETTE_SIZE)

	for i := range PALETTE_SIZE {
		palette[i] = color.RGBA{data[i*3], data[i*3+1], data[i*3+2], 255}
	}

	return palette, nil
}

func find_closest(c color.Color, p color.Palette) color.RGBA {
	cr, cg, cb, _ := c.RGBA()
	min_distance := math.MaxFloat64
	var closest color.RGBA

	for _, pcolor := range p {
		pr, pg, pb, _ := pcolor.RGBA()
		distancer := float64(pr>>8) - float64(cr>>8)
		distanceg := float64(pg>>8) - float64(cg>>8)
		distanceb := float64(pb>>8) - float64(cb>>8)

		// applying euclidean distance without square root
		// the constants multpliying the distance^2 is the weight of each hue
		// to the human eye, the most noticable hues are: green, red and then blue
		distance := math.Sqrt(2*distancer*distancer + 3*distanceg*distanceg + 1*distanceb*distanceb)

		if distance < min_distance {
			min_distance = distance
			closest = color.RGBA{uint8(pr >> 8), uint8(pg >> 8), uint8(pb >> 8), 255}
		}
	}

	return closest
}

func has_palette(img image.Image, p color.Palette) bool {
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := find_closest(img.At(x, y), p)
			_rc, _gc, _bc, _ := img.At(x, y).RGBA()
			rc, gc, bc := uint8(_rc>>8), uint8(_gc>>8), uint8(_bc>>8)

			if rc != c.R || gc != c.G || bc != c.B {
				return false
			}
		}
	}

	return true
}

func identify(img image.Image) (int, error) {
	dir := "palettes"
	entries, err := palettes.ReadDir(dir)
	if err != nil {
		return 1, err
	}

	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() || filepath.Ext(filename) != ".pal" {
			continue
		}

		p, err := load_palette(filepath.Join(dir, entry.Name()))
		if err != nil {
			return 1, err
		}

		if has_palette(img, p) {
			println("The palette used in this image was:", strings.TrimSuffix(filename, ".pal"))
			return 0, nil
		}
	}

	println("No palette matches this image colorscheme")
	return 0, nil
}

func remap(img image.Image) (int, error) {
	palette, err := load_palette(os.Args[3])
	if err != nil {
		return 1, err
	}

	remappedf, err := os.Create(os.Args[4])
	if err != nil {
		remappedf.Close()
		return 1, err
	}
	defer remappedf.Close()

	bounds := img.Bounds()
	remapped := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			remapped.Set(x, y, find_closest(img.At(x, y), palette))
		}
	}

	switch filepath.Ext(remappedf.Name())[1:] {
	case "png":
		err = png.Encode(remappedf, remapped)
	case "jpg", "jpeg":
		err = jpeg.Encode(remappedf, remapped, nil)
	default:
		return 2, errors.New("Output type is not a supported format")
	}

	if err != nil {
		return 1, err
	}
	return 0, nil
}

func run() int {
	log.SetFlags(0)

	ex, err := os.Executable()
	if err != nil {
		log.Println(err)
		return 1
	}

	ex_name := filepath.Base(ex)

	type Command struct {
		Desc  string
		Usage string
		Doc   string
	}

	// TODO: Make so that LIST can differentiate the variant palettes
	cmd_names := [...]string{"identify", "remap", "list"}
	cmds := map[string]Command{
		cmd_names[0]: {
			Desc:  "analyzes an image and identifies the color palette used",
			Usage: fmt.Sprintf("%s %s <image> [palette...]", ex_name, cmd_names[0]),
			Doc: fmt.Sprintf(strings.TrimSuffix(strings.ReplaceAll(`
					Analyzes an image and identifies the color palette used.
					The output is the found color palette name in the default palette list,
					this list can be shown with '%s %s'.
					Optionally, you may enter one or more palettes to match instead of the
					default palette list.
				`, "\t", ""), "\n"), ex_name, cmd_names[2])[1:],
		},
		cmd_names[1]: {
			Desc:  "replaces the colors in a image using a color palette",
			Usage: fmt.Sprintf("%s %s <image> [flags] <palette> <output_image>", ex_name, cmd_names[1]),
			Doc: strings.TrimSuffix(strings.ReplaceAll(`
					Replaces the colors in a image using a color palette
				`, "\t", ""), "\n")[1:],
		},
		cmd_names[2]: {
			Desc:  "displays the default palette list",
			Usage: fmt.Sprintf("%s %s", ex_name, cmd_names[2]),
			Doc: strings.TrimSuffix(strings.ReplaceAll(`
					Displays the default palette list
				`, "\t", ""), "\n")[1:],
		},
	}
	get_cmd_list := func(cmds map[string]Command) []string {
		output := make([]string, 0, len(cmds))

		max_padding := 0
		for k := range cmds {
			if len(k) > max_padding {
				max_padding = len(k)
			}
		}

		for k, cmd := range cmds {
			output = append(output, k+strings.Repeat(" ", max_padding-len(k)+2)+cmd.Desc)
		}

		sort.Strings(output)
		return output
	}

	help := strings.TrimSuffix(fmt.Sprintf(`
Nespal is a tool for manipulating images using color palettes from the Nintendo Entertainment System (NES) emulation ecosystem

Usage: %s <command> <image>... [options] [output]

The commands are:
	%s

Use "%s help <command>" for more information about a command
	`, ex_name, strings.Join(get_cmd_list(cmds), "\n\t"), ex_name), "\n\t")[1:]

	try_help := fmt.Sprintf("Try: %s help", ex_name)

	if len(os.Args) == 1 {
		println(help)
		log.Printf("\n%s: missing command\n", ex_name)
		return 2
	}

	if _, ok := cmds[os.Args[1]]; !ok && os.Args[1] != "help" {
		log.Printf("%s: unknown command \"%s\"\n", ex_name, os.Args[1])
		log.Printf(try_help)
		return 2
	}

	load_image := func(filename string) (image.Image, error) {
		sourcef, err := os.Open(filename)
		if err != nil {
			sourcef.Close()
			return nil, err
		}
		defer sourcef.Close()

		source, _, err := image.Decode(sourcef)
		if err != nil {
			return nil, err
		}

		return source, nil
	}

	switch os.Args[1] {
	case "help":
		if len(os.Args) == 2 {
			println(help)
			return 0
		}

		cmd, ok := cmds[os.Args[2]]
		if !ok {
			log.Printf("%s: unknown help topic \"%s\"\n", ex_name, os.Args[2])
			log.Println(try_help)
			return 2
		}

		fmt.Printf("Usage: %s\n\n", cmd.Usage)
		println(cmd.Doc)
		return 0
	case cmd_names[0]:
		source, err := load_image(os.Args[2])
		if err != nil {
			log.Println(err)
			return 1
		}

		status, err := identify(source)
		if err != nil {
			log.Println(err)
			return status
		}
	case cmd_names[1]:
		source, err := load_image(os.Args[2])
		if err != nil {
			log.Println(err)
			return 1
		}

		if len(os.Args) < 4 {
			log.Printf("%s: missing color palette\n", ex_name)
			return 2
		}

		if filepath.Ext(os.Args[3]) != ".pal" {
			log.Printf("%s: nsupported palette file format for '%s', expected '.pal'\n", ex_name, os.Args[3])
			return 2
		}

		status, err := remap(source)
		if err != nil {
			log.Println(err)
			return status
		}
	case cmd_names[2]:
		if err = fs.WalkDir(palettes, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				println(filepath.Base(path))
			}

			return nil
		}); err != nil {
			log.Println(err)
			return 1
		}
	}

	return 0
}

func main() {
	os.Exit(run())
}
