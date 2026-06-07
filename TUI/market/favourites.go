package market

func (s *Store) SeedFavourites(coins []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range coins {
		if !s.inList[c] {
			s.order = append(s.order, c)
			s.inList[c] = true
		}
		s.favourites[c] = true
		s.seedSnapshot(c)
	}
}

func (s *Store) seedSnapshot(coin string) {
	l, ok := s.uni.Get(coin)
	if !ok {
		return
	}
	if _, seen := s.ctx[coin]; !seen && l.Snapshot.MarkPx != "" {
		s.ctx[coin] = l.Snapshot
	}
}

func (s *Store) WatchedCoins() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

func (s *Store) WatchedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.order)
}

func (s *Store) IsWatched(coin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inList[coin]
}

func (s *Store) IsFavourite(coin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.favourites[coin]
}

func (s *Store) Add(coin string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inList[coin] {
		return false
	}
	s.order = append(s.order, coin)
	s.inList[coin] = true
	s.seedSnapshot(coin)
	return true
}

func (s *Store) Remove(coin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.inList[coin] {
		return
	}
	delete(s.inList, coin)
	delete(s.favourites, coin)
	for i, c := range s.order {
		if c == coin {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// ToggleFavourite flips favourite and returns the new favourite state
func (s *Store) ToggleFavourite(coin string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.favourites[coin] {
		delete(s.favourites, coin)
		return false
	}
	s.favourites[coin] = true
	if !s.inList[coin] {
		s.order = append(s.order, coin)
		s.inList[coin] = true
		s.seedSnapshot(coin)
	}
	return true
}

func (s *Store) Move(coin string, delta int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, c := range s.order {
		if c == coin {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	j := idx + delta
	if j < 0 || j >= len(s.order) {
		return false
	}
	s.order[idx], s.order[j] = s.order[j], s.order[idx]
	return true
}

// Favourites returns favourited coins in display order
func (s *Store) Favourites() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.favourites))
	for _, c := range s.order {
		if s.favourites[c] {
			out = append(out, c)
		}
	}
	return out
}
