package client

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/andreykaipov/goobs/api/requests/filters"
)

type filterSnapshot struct {
	source, name, kind string
	settings           map[string]any
	index              int
	enabled            bool
}

func (o RecordingSessionOBS) snapshotFilter(source, name string) (filterSnapshot, error) {
	r, err := o.Client.Filters.GetSourceFilter(filters.NewGetSourceFilterParams().WithSourceName(source).WithFilterName(name))
	if err != nil {
		return filterSnapshot{}, err
	}
	if r.FilterKind != "source_record_filter" || r.FilterSettings == nil || r.FilterIndex < 0 {
		return filterSnapshot{}, fmt.Errorf("cannot snapshot Source Record filter %q on %q", name, source)
	}
	return filterSnapshot{source, name, r.FilterKind, r.FilterSettings, r.FilterIndex, r.FilterEnabled}, nil
}

func (o RecordingSessionOBS) filterExists(s filterSnapshot) (bool, error) {
	r, err := o.Client.Filters.GetSourceFilterList(filters.NewGetSourceFilterListParams().WithSourceName(s.source))
	if err != nil {
		return false, err
	}
	for _, filter := range r.Filters {
		if filter != nil && filter.FilterName == s.name {
			return true, nil
		}
	}
	return false, nil
}

// restoreFilter also handles ambiguous Remove/Create replies: query existence
// instead of blindly removing a possibly healthy filter or creating duplicates.
func (o RecordingSessionOBS) restoreFilter(s filterSnapshot, guard func() error) error {
	exists, err := o.filterExists(s)
	if err != nil {
		return err
	}
	if !exists {
		if err := guard(); err != nil {
			return err
		}
		_, err = o.Client.Filters.CreateSourceFilter(filters.NewCreateSourceFilterParams().
			WithSourceName(s.source).WithFilterName(s.name).WithFilterKind(s.kind).WithFilterSettings(s.settings))
		if err != nil {
			// The request may have reached OBS. Continue recovery if it exists now,
			// but retain the error so configuration can never proceed to start.
			present, readErr := o.filterExists(s)
			if readErr != nil || !present {
				return errors.Join(err, readErr)
			}
		}
	} else {
		current, readErr := o.snapshotFilter(s.source, s.name)
		if readErr != nil {
			return readErr
		}
		if reflect.DeepEqual(current, s) {
			return nil
		} // Failed removal was a no-op.
		return fmt.Errorf("filter %q on %q changed during reset; refusing to overwrite it", s.name, s.source)
	}
	// Continue independent restoration operations after an error, best effort.
	if guardErr := guard(); guardErr != nil {
		return errors.Join(err, guardErr)
	}
	_, indexErr := o.Client.Filters.SetSourceFilterIndex(filters.NewSetSourceFilterIndexParams().
		WithSourceName(s.source).WithFilterName(s.name).WithFilterIndex(s.index))
	err = errors.Join(err, indexErr)
	if guardErr := guard(); guardErr != nil {
		return errors.Join(err, guardErr)
	}
	_, enabledErr := o.Client.Filters.SetSourceFilterEnabled(filters.NewSetSourceFilterEnabledParams().
		WithSourceName(s.source).WithFilterName(s.name).WithFilterEnabled(s.enabled))
	err = errors.Join(err, enabledErr)
	current, readErr := o.snapshotFilter(s.source, s.name)
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	if !reflect.DeepEqual(current, s) {
		err = errors.Join(err, fmt.Errorf("restored filter %q on %q did not match snapshot", s.name, s.source))
	}
	return err
}

// WithDetachedFilters destroys Source Record's private views before resetting
// OBS video. Both complete snapshots precede any removal. Recovery includes an
// ambiguously failed removal and always attempts both sources, without starting.
func (o RecordingSessionOBS) WithDetachedFilters(sources []string, name string, guard func() error, change func() error) (err error) {
	if len(sources) != 2 || sources[0] == sources[1] {
		return errors.New("video reset requires two distinct Source Record sources")
	}
	snapshots := make([]filterSnapshot, 0, 2)
	for _, source := range sources {
		s, readErr := o.snapshotFilter(source, name)
		if readErr != nil {
			return fmt.Errorf("snapshot filter on %q: %w", source, readErr)
		}
		snapshots = append(snapshots, s)
	}
	attempted := 0
	defer func() {
		for _, s := range snapshots[:attempted] {
			if restoreErr := o.restoreFilter(s, guard); restoreErr != nil {
				err = errors.Join(err, fmt.Errorf("restore Source Record filter on %q: %w", s.source, restoreErr))
			}
		}
		if attempted > 0 {
			// Let recreated private views settle before a recording can start.
			time.Sleep(500 * time.Millisecond)
		}
	}()
	for _, s := range snapshots {
		if err := guard(); err != nil {
			return err
		}
		attempted++ // A failed reply may still have destroyed the private view.
		if _, err := o.Client.Filters.RemoveSourceFilter(filters.NewRemoveSourceFilterParams().WithSourceName(s.source).WithFilterName(s.name)); err != nil {
			return fmt.Errorf("remove Source Record filter on %q: %w", s.source, err)
		}
	}
	// obs_source_release/graphics destruction is queued, not synchronous with RPC.
	time.Sleep(500 * time.Millisecond)
	if err := guard(); err != nil {
		return err
	}
	return change()
}
