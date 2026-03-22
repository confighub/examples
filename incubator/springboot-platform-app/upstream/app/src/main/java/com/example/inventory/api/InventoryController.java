package com.example.inventory.api;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/inventory")
public class InventoryController {

  private final InventoryService inventoryService;
  private final InventoryFeatureProperties featureProperties;
  private final InventoryRuntimeProperties runtimeProperties;
  private final String serviceName;
  private final String cacheBackend;

  public InventoryController(
      InventoryService inventoryService,
      InventoryFeatureProperties featureProperties,
      InventoryRuntimeProperties runtimeProperties,
      @Value("${spring.application.name:inventory-api}") String serviceName,
      @Value("${CACHE_BACKEND:none}") String cacheBackend) {
    this.inventoryService = inventoryService;
    this.featureProperties = featureProperties;
    this.runtimeProperties = runtimeProperties;
    this.serviceName = serviceName;
    this.cacheBackend = cacheBackend;
  }

  @GetMapping("/items")
  public ResponseEntity<?> listItems() {
    return ResponseEntity.ok(inventoryService.listItems());
  }

  @GetMapping("/items/{sku}")
  public ResponseEntity<?> getItem(@PathVariable String sku) {
    return inventoryService.findItem(sku)
        .<ResponseEntity<?>>map(ResponseEntity::ok)
        .orElseGet(() -> ResponseEntity.notFound().build());
  }

  @GetMapping("/summary")
  public ResponseEntity<InventorySummary> summary() {
    return ResponseEntity.ok(
        new InventorySummary(
            serviceName,
            runtimeProperties.getEnvironment(),
            featureProperties.getReservationMode(),
            cacheBackend,
            inventoryService.listItems()));
  }
}
