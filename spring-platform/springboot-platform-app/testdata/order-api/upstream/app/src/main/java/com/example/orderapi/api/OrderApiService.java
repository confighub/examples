package com.example.orderapi.api;

import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Service;

@Service
public class OrderApiService {

  private static final List<OrderApiItem> ITEMS =
      List.of(
          new OrderApiItem("SKU-100", "widget", 12),
          new OrderApiItem("SKU-200", "cable", 4),
          new OrderApiItem("SKU-300", "sensor", 19));

  public List<OrderApiItem> listItems() {
    return ITEMS;
  }

  public Optional<OrderApiItem> findItem(String sku) {
    return ITEMS.stream()
        .filter(item -> item.sku().equalsIgnoreCase(sku))
        .findFirst();
  }
}
