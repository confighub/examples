package com.example.orderapi.api;

import java.util.List;

public record OrderApiSummary(
    String service,
    String environment,
    String reservationMode,
    String cacheBackend,
    List<OrderApiItem> items) {}
