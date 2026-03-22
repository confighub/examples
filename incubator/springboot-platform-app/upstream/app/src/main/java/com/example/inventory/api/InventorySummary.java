package com.example.inventory.api;

import java.util.List;

public record InventorySummary(
    String service,
    String environment,
    String reservationMode,
    String cacheBackend,
    List<InventoryItem> items) {}
