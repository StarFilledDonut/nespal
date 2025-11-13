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
	"unicode"

	"github.com/spf13/pflag"
)

type Command struct {
	Desc  string
	Usage string
	Doc   string
}

const (
	IDENTIFY = "identify"
	REMAP    = "remap"
	LIST     = "list"
	HELP     = "help"
)

var (
	//go:embed palettes/**.pal
	palettes embed.FS
	ex       string
)

// TODO: Make tests
// TODO: Detect wrong types of .pal

// Extracts the color palette from an NES/FAMICOM pal file
// will not work with pal files for other uses
func load_palette(pal io.Reader) (color.Palette, error) {
	const PALETTE_SIZE = 64

	// NES color palette has 64 colors in RGB format
	data := make([]byte, PALETTE_SIZE*3)
	_, err := io.ReadFull(pal, data)
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

func identify(img image.Image, custom_pals []*os.File, custom_only bool) (int, error) {
	for _, pal := range custom_pals {
		p, err := load_palette(pal)
		if err != nil {
			return 1, err
		}

		if has_palette(img, p) {
			println("The palette used in this image was:", strings.TrimSuffix(pal.Name(), ".pal"))
			return 0, nil
		}
	}

	if custom_only && len(custom_pals) > 0 {
		return 0, nil
	} else if custom_only {
		return 2, fmt.Errorf("%s: flag 'custom-only' reguires input color palettes", ex)
	}

	entries, err := fs.ReadDir(palettes, "palettes")
	if err != nil {
		return 1, err
	}

	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() || filepath.Ext(filename) != ".pal" {
			continue
		}
		file, err := palettes.Open(filepath.Join("palettes", filename))
		if err != nil {
			return 1, err
		}

		p, err := load_palette(file)
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

func remap(img image.Image, pal io.Reader, dst_path string) (int, error) {
	p, err := load_palette(pal)
	if err != nil {
		return 1, err
	}

	remappedf, err := os.Create(dst_path)
	if err != nil {
		remappedf.Close()
		return 1, err
	}
	defer remappedf.Close()

	bounds := img.Bounds()
	remapped := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			remapped.Set(x, y, find_closest(img.At(x, y), p))
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

func get_commands() map[string]Command {
	// TODO: Make so that LIST can differentiate the variant palettes
	// TODO: Complete the documentation of each command
	cmds := map[string]Command{
		IDENTIFY: {
			Desc:  "analyzes an image and identifies the color palette used",
			Usage: fmt.Sprintf("%s %s <image> [palette...]", ex, IDENTIFY),
			Doc: fmt.Sprintf(strings.TrimSuffix(strings.ReplaceAll(`
					Analyzes an image and identifies the color palette used.
					The output is the found color palette in the default palette list,
					this list can be shown with '%s %s'.
					Optionally, you may enter one or more palettes to match instead of the
					default palette list.
				`, "\t", ""), "\n"), ex, IDENTIFY)[1:],
		},
		REMAP: {
			Desc:  "replaces the colors in a image using a color palette",
			Usage: fmt.Sprintf("%s %s <image> [flags] <palette> <output_image>", ex, REMAP),
			Doc: strings.TrimSuffix(strings.ReplaceAll(`
					Replaces the colors in a image using a color palette
				`, "\t", ""), "\n")[1:],
		},
		LIST: {
			Desc:  "displays the default palette list",
			Usage: fmt.Sprintf("%s %s", ex, LIST),
			Doc: strings.TrimSuffix(strings.ReplaceAll(`
					Displays the default palette list
				`, "\t", ""), "\n")[1:],
		},
	}
	return cmds
}

func get_help(cmds map[string]Command) string {
	cmd_list := make([]string, 0, len(cmds))

	max_padding := 0
	for k := range cmds {
		if len(k) > max_padding {
			max_padding = len(k)
		}
	}

	for k, cmd := range cmds {
		cmd_list = append(cmd_list, k+strings.Repeat(" ", max_padding-len(k)+2)+cmd.Desc)
	}

	sort.Strings(cmd_list)

	return strings.TrimSuffix(fmt.Sprintf(`
Nespal is a tool for manipulating images using color palettes from the Nintendo Entertainment System (NES) emulation ecosystem

Usage: %s <command> <image>... [options] [output]

The commands are:
	%s

Use "%s %s <command>" for more information about a command
	`, ex, strings.Join(cmd_list, "\n\t"), ex, HELP), "\n\t")[1:]
}

func run() int {
	cmds := get_commands()
	help := get_help(cmds)
	try_help := fmt.Sprintf("Try: %s %s", ex, HELP)
	args := os.Args[1:]

	if len(args) == 0 {
		println(help)
		log.Printf("\n%s: missing command\n", ex)
		return 2
	}

	if _, ok := cmds[args[0]]; !ok && args[0] != HELP {
		log.Printf("%s: unknown command \"%s\"\n", ex, os.Args[1])
		log.Printf(try_help)
		return 2
	}

	load_image := func(fil string) (image.Image, error) {
		sourcef, err := os.Open(fil)
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

	switch args[0] {
	case IDENTIFY:
		custom_only := pflag.BoolP("custom-only", "c", false, "Only match against input color palettes")
		pflag.Parse()
		args = pflag.Args()

		if len(args) == 1 {
			log.Printf("%s: missing image file\n", ex)
			return 2
		}

		source, err := load_image(args[1])
		if err != nil {
			log.Println(err)
			return 1
		}
		custom_pals := make([]*os.File, len(args)-2)
		for i := range custom_pals {
			path := args[i+2]
			if filepath.Ext(path) != ".pal" {
				log.Printf("%s: usupported palette file format for '%s', expected '.pal'\n", ex, os.Args[3])
				return 2
			}

			file, err := os.Open(path)
			if err != nil {
				log.Println(err)
				return 1
			}
			custom_pals[i] = file
		}

		if status, err := identify(source, custom_pals, *custom_only); err != nil {
			log.Println(err)
			return status
		}
	case REMAP:
		chosen_pal := pflag.StringP("palette", "p", "", "Color palette to remap image to")
		pflag.Parse()
		args = pflag.Args()

		if len(args) == 1 {
			log.Printf("%s: missing image file\n", ex)
			return 2
		}

		source, err := load_image(args[1])
		if err != nil {
			log.Println(err)
			return 1
		}

		if *chosen_pal != "" {
			res := make([]rune, 0, len(*chosen_pal))
			for _, r := range *chosen_pal {
				if !unicode.IsSpace(r) {
					res = append(res, r)
				}
			}
			if string(res) == "" {
				log.Printf("%s: empty value for '--palette' flag", ex)
				return 2
			}

			if strings.Contains(*chosen_pal, ".") {
				log.Printf("%s: invalid value '%s' for '--palette' flag", ex, *chosen_pal)
				return 2
			}

			var pal fs.File = nil
			entries, err := fs.ReadDir(palettes, "palettes")
			if err != nil {
				log.Println(err)
				return 1
			}

			for _, d := range entries {
				if d.IsDir() {
					continue
				}

				if strings.HasSuffix(d.Name(), ".pal") && strings.EqualFold(*chosen_pal, strings.TrimSuffix(d.Name(), ".pal")) {
					pal, err = palettes.Open(filepath.Join("palettes", d.Name()))
					if err != nil {
						log.Println(err)
						return 1
					}
				}
			}

			if pal == nil {
				log.Printf("%s: palette '%s' not in the palette list", ex, *chosen_pal)
				return 2
			}

			if len(args) == 2 {
				log.Printf("%s: missing output image\n", ex)
				return 2
			}

			if status, err := remap(source, pal, args[2]); err != nil {
				log.Println(err)
				return status
			}
			return 0
		}

		if len(args) == 2 {
			log.Printf("%s: missing color palette\n", ex)
			return 2
		}

		if filepath.Ext(args[2]) != ".pal" {
			log.Printf("%s: usupported palette file format for '%s', expected '.pal'\n", ex, args[2])
			return 2
		}

		if len(args) == 3 {
			log.Printf("%s: missing output image\n", ex)
			return 2
		}

		input_pal, err := os.Open(args[2])
		if err != nil {
			log.Println(err)
			return 1
		}

		if status, err := remap(source, input_pal, args[3]); err != nil {
			log.Println(err)
			return status
		}
	case LIST:
		if err := fs.WalkDir(palettes, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			pal, found := strings.CutSuffix(d.Name(), ".pal")
			if !found {
				return nil
			}
			println(pal)

			return nil
		}); err != nil {
			log.Println(err)
			return 1
		}
	case HELP:
		if len(os.Args) == 2 {
			println(help)
			return 0
		}

		cmd, ok := cmds[os.Args[2]]
		if !ok {
			log.Printf("%s: unknown help topic \"%s\"\n", ex, os.Args[2])
			log.Println(try_help)
			return 2
		}

		fmt.Printf("Usage: %s\n\n", cmd.Usage)
		println(cmd.Doc)
		return 0
	}

	return 0
}

func init() {
	log.SetFlags(0)
	ex = filepath.Base(os.Args[0])
}

func main() {
	os.Exit(run())
}
