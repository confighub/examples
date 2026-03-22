package com.example.inventory.api;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "feature.inventory")
public class InventoryFeatureProperties {

  private String reservationMode = "optimistic";

  public String getReservationMode() {
    return reservationMode;
  }

  public void setReservationMode(String reservationMode) {
    this.reservationMode = reservationMode;
  }
}
