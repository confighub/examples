package com.example.inventory.api;

import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Service;

@Service
public class InventoryService {

  private static final List<InventoryItem> ITEMS =
      List.of(
          new InventoryItem("SKU-100", "widget", 12),
          new InventoryItem("SKU-200", "cable", 4),
          new InventoryItem("SKU-300", "sensor", 19));

  public List<InventoryItem> listItems() {
    return ITEMS;
  }

  public Optional<InventoryItem> findItem(String sku) {
    return ITEMS.stream()
        .filter(item -> item.sku().equalsIgnoreCase(sku))
        .findFirst();
  }
}
