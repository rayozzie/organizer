package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dsoprea/go-exif"
)

type ExifDataEntry struct {
	IfdPath     string                `json:"ifd_path"`
	FqIfdPath   string                `json:"fq_ifd_path"`
	IfdIndex    int                   `json:"ifd_index"`
	TagId       uint16                `json:"tag_id"`
	TagName     string                `json:"tag_name"`
	TagTypeId   exif.TagTypePrimitive `json:"tag_type_id"`
	TagTypeName string                `json:"tag_type_name"`
	UnitCount   uint32                `json:"unit_count"`
	Value       interface{}           `json:"value"`
	ValueString string                `json:"value_string"`
}

func exifExtract(filePath string) (exifData map[string]ExifDataEntry) {
	exifData = map[string]ExifDataEntry{}

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return exifData
	}

	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return exifData
	}

	rawExif, err := exif.SearchAndExtractExif(data)
	if err != nil {
		if err.Error() != exif.ErrNoExif.Error() {
			fmt.Printf("Error extracting EXIF data (%s): %v\n", filePath, err)
		}
		return exifData
	}

	// Run the parse.

	im := exif.NewIfdMappingWithStandard()
	ti := exif.NewTagIndex()

	visitor := func(fqIfdPath string, ifdIndex int, tagId uint16, tagType exif.TagType, valueContext exif.ValueContext) (err error) {
		defer func() {
			if state := recover(); state != nil {
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
			}
		}()

		ifdPath, err := im.StripPathPhraseIndices(fqIfdPath)
		if err != nil {
			fmt.Printf("Error stripping path-phrase-indices: %v\n", err)
			return
		}

		it, err := ti.Get(ifdPath, tagId)
		if err != nil {
			if err.Error() == exif.ErrTagNotFound.Error() {
				if false {
					fmt.Printf("WARNING: Unknown tag: [%s] (%04x)\n", ifdPath, tagId)
				}
				return nil
			} else {
				fmt.Printf("Error stripping path-phrase-indices: %v\n", err)
				return
			}
		}

		valueString := ""
		var value interface{}
		if tagType.Type() == exif.TypeUndefined {
			var err error
			value, err = valueContext.Undefined()
			if err != nil {
				if err == exif.ErrUnhandledUnknownTypedTag {
					value = nil
				} else {
					log.Panic(err)
				}
			}

			valueString = fmt.Sprintf("%v", value)
		} else {
			valueString, err = valueContext.FormatFirst()
			if err != nil {
				fmt.Printf("Error formatting: %v\n", err)
				return
			}

			value = valueString
		}

		entry := ExifDataEntry{
			IfdPath:     ifdPath,
			FqIfdPath:   fqIfdPath,
			IfdIndex:    ifdIndex,
			TagId:       tagId,
			TagName:     it.Name,
			TagTypeId:   tagType.Type(),
			TagTypeName: tagType.Name(),
			UnitCount:   valueContext.UnitCount(),
			Value:       value,
			ValueString: valueString,
		}

		exifData[it.Name] = entry
		return nil
	}

	_, err = exif.Visit(exif.IfdStandard, im, ti, rawExif, visitor)
	if err != nil {
		fmt.Printf("Error visiting: %v\n", err)
		return exifData
	}
	return exifData
}
