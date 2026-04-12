package core

import "strings"

// rebuildIndices repopulates the idByFile and fileByID maps from the
// current Manifest.Slides slice. Call after OpenDeck loads the manifest
// and after every mutation that changes the Slides records.
func (d *Deck) rebuildIndices() {
	d.idByFile = make(map[string]string)
	d.fileByID = make(map[string]string)
	if d.Manifest == nil {
		return
	}
	for _, rec := range d.Manifest.Slides {
		d.idByFile[rec.File] = rec.ID
		d.fileByID[rec.ID] = rec.File
	}
}

// SlideIDForFile returns the slide_id assigned to the given filename,
// or "" if the file doesn't have an id (legacy deck not yet mutated).
func (d *Deck) SlideIDForFile(filename string) string {
	return d.idByFile[filename]
}

// ensureSlideIDs generates slide_ids for any slide on disk that doesn't
// yet have one in the manifest. This is the auto-migration path:
// legacy decks (pre-#83) with no `slides:` section get IDs assigned
// transparently on the first mutation. The method is idempotent —
// running it twice in a row is a no-op on the second call.
//
// Also prunes stale records whose files no longer exist on disk,
// healing the manifest if a prior crash left it out of sync.
//
// This method modifies Manifest.Slides in place and rebuilds the
// in-memory indices. The caller is responsible for calling saveManifest
// afterward if the changes should persist.
func (d *Deck) ensureSlideIDs() error {
	if d.Manifest == nil {
		d.Manifest = &Manifest{}
	}

	slides, err := d.SlideFilenames()
	if err != nil {
		return err
	}

	// Files already known.
	known := make(map[string]bool, len(d.Manifest.Slides))
	for _, rec := range d.Manifest.Slides {
		known[rec.File] = true
	}

	// IDs already used (for collision avoidance).
	usedIDs := make(map[string]bool, len(d.Manifest.Slides))
	for _, rec := range d.Manifest.Slides {
		usedIDs[rec.ID] = true
	}

	// Assign IDs to new files.
	for _, f := range slides {
		if known[f] {
			continue
		}
		id := uniqueSlideID(usedIDs)
		d.Manifest.Slides = append(d.Manifest.Slides, SlideRecord{
			ID:   id,
			File: f,
		})
	}

	// Prune stale records (file no longer on disk).
	onDisk := make(map[string]bool, len(slides))
	for _, f := range slides {
		onDisk[f] = true
	}
	kept := d.Manifest.Slides[:0]
	for _, rec := range d.Manifest.Slides {
		if onDisk[rec.File] {
			kept = append(kept, rec)
		}
	}
	d.Manifest.Slides = kept

	d.rebuildIndices()
	return nil
}

// updateSlideFilenames updates the File field of every SlideRecord whose
// old filename changed after a RewriteSlideOrder or SlugifySlides rename
// pass. The renames map is oldFilename → newFilename. Records for files
// not in the renames map are left unchanged. Rebuilds indices after.
func (d *Deck) updateSlideFilenames(renames map[string]string) {
	for i := range d.Manifest.Slides {
		if newName, ok := renames[d.Manifest.Slides[i].File]; ok {
			d.Manifest.Slides[i].File = newName
		}
	}
	d.rebuildIndices()
}

// saveManifest writes Manifest to .slyds.yaml on the Deck's filesystem.
// Call after every successful mutation to persist id→file mapping changes.
// Returns nil without writing if Manifest is nil (legacy deck with no
// config file at all — mutation paths init the manifest via ensureSlideIDs
// so this case effectively can't happen after the first mutation).
func (d *Deck) saveManifest() error {
	if d.Manifest == nil {
		return nil
	}
	return WriteManifestFS(d.FS, *d.Manifest)
}

// slideSlugFromFile extracts the slug portion of a slide filename.
// Utility that combines ExtractNamePart and the .html suffix strip
// in a single call.
func slideSlugFromFile(filename string) string {
	return strings.TrimSuffix(ExtractNamePart(filename), ".html")
}
