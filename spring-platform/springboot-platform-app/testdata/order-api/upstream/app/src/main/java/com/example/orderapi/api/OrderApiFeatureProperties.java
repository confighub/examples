package com.example.orderapi.api;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "feature.inventory")
public class OrderApiFeatureProperties {

  private String reservationMode = "optimistic";

  public String getReservationMode() {
    return reservationMode;
  }

  public void setReservationMode(String reservationMode) {
    this.reservationMode = reservationMode;
  }
}
