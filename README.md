# nespal

`nespal` is a program to manipulate images using color palettes from the NES ecosystem.

[Installation](#installation) • [Usage](#usage)

## Features

* Simple and straight-forward syntax.
* Remaps images with color palettes stored in `.pal` files.
* Indetifies wheanever a image uses a NES color palette and detects which has been used.
* Converts your image colorscheme into a color palette, if that image has the minimum of colors needed.
* Create a color palette preview image.
* Has a wide variety of pre-built color palettes.

## Usage

The general docummentation of *nespal* can be showed with `go help`, for a more in-depth documentation
of a specific command, use `go help <command>`.

### Identify color palette

Detects the NES color palette used by the image, can extend the list with a set of color palettes

```bash
nespal identify <image> [palette...]
```

The pre-built palettes can be excluded from the comparassion list with `--custom-only` or `-c`

### Remapping images

Remap a image using a color palette

```bash
nespal remap <image> <palette> <ouput_image>
```

The color palette can either be a file, or a pre-built palette with `--palette='fceux'` or `-p='fceux'`

### Listing available color palettes

Pre-built palettes can be displayed and sorted

```bash
nespal list
```

## Installation

With golang package manager, you can install *nespal* via:

```bash
go install github/StarFilledDonut/nespal@latest
```

## Build source

```bash
git clone https://github.com/StarFilledDonut/nespal
cd nespal
go build
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE)

## Third-party licences

The color palettes in `palettes/` are derived from 
[game tech wiki](https://emulation.gametechwiki.com/index.php/Famicom_color_palette) 
and are licensed under the GNU General Public License v3.

See `palettes/COPYING` for the full license text.
