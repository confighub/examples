package com.example.inventory.api;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.test.context.ActiveProfiles;

@ActiveProfiles("prod")
@SpringBootTest(
    webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT,
    properties = {"CACHE_BACKEND=redis"})
class InventoryControllerProdHttpTest {

  @LocalServerPort private int port;

  @Autowired private TestRestTemplate restTemplate;

  @Test
  void summaryEndpointReflectsProdProfileAndCacheBackendOverHttp() {
    InventorySummary response =
        restTemplate.getForObject(baseUrl() + "/api/inventory/summary", InventorySummary.class);

    assertThat(response).isNotNull();
    assertThat(response.environment()).isEqualTo("prod");
    assertThat(response.reservationMode()).isEqualTo("strict");
    assertThat(response.cacheBackend()).isEqualTo("redis");
  }

  private String baseUrl() {
    return "http://localhost:" + port;
  }
}
