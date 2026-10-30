package main

import "testing"

// TestDeleteCoinCascadesPrices проверяет ON DELETE CASCADE: удаление монеты
// должно удалять всю её ценовую историю.
func TestDeleteCoinCascadesPrices(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	c, err := createCoin(db, "BTC", "Bitcoin", "USD", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := insertPrice(db, c.ID, float64(i+1), "USD", "2026-01-01T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}

	if n, err := countPrices(db, c.ID, "", ""); err != nil || n != 3 {
		t.Fatalf("цен до удаления = %d, %v (want 3)", n, err)
	}

	ok, err := deleteCoin(db, c.ID)
	if err != nil || !ok {
		t.Fatalf("delete = %v, %v", ok, err)
	}

	if n, err := countPrices(db, c.ID, "", ""); err != nil || n != 0 {
		t.Fatalf("цен после удаления = %d, %v (want 0 — каскад не сработал)", n, err)
	}

	// Прямая проверка на уровне SQL (не через репозиторий).
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM price_history WHERE coin_id = $1", c.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("в price_history осталось %d строк для coin_id=%d", total, c.ID)
	}
}

// TestDeleteCoinUnknown возвращает false без ошибки.
func TestDeleteCoinUnknown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ok, err := deleteCoin(db, 999999)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("delete несуществующей монеты вернул ok=true")
	}
}
