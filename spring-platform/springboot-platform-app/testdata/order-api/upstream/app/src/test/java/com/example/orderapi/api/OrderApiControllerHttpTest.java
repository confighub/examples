package com.example.orderapi.api;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class OrderApiControllerHttpTest {

  @LocalServerPort private int port;

  @Autowired private TestRestTemplate restTemplate;

  @Test
  void itemsEndpointReturnsSeedDataOverHttp() {
    ResponseEntity<OrderApiItem[]> response =
        restTemplate.getForEntity(baseUrl() + "/api/inventory/items", OrderApiItem[].class);

    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(response.getBody()).isNotNull();
    assertThat(response.getBody()).hasSize(3);
    assertThat(response.getBody()[0].sku()).isEqualTo("SKU-100");
  }

  @Test
  void summaryEndpointReturnsDefaultRuntimeValuesOverHttp() {
    ResponseEntity<OrderApiSummary> response =
        restTemplate.getForEntity(baseUrl() + "/api/inventory/summary", OrderApiSummary.class);

    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(response.getBody()).isNotNull();
    assertThat(response.getBody().service()).isEqualTo("order-api");
    assertThat(response.getBody().environment()).isEqualTo("dev");
    assertThat(response.getBody().reservationMode()).isEqualTo("optimistic");
    assertThat(response.getBody().cacheBackend()).isEqualTo("none");
    assertThat(response.getBody().items()).hasSize(3);
  }

  private String baseUrl() {
    return "http://localhost:" + port;
  }
}
