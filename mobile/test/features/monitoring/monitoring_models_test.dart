import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/monitoring/data/monitoring_models.dart';

void main() {
  test(
    'uses the shared backend fixture for the same RT list and total as Web',
    () {
      final sharedResponse =
          jsonDecode(
                File(
                  '../frontend/src/testdata/monitoring-api-response.json',
                ).readAsStringSync(),
              )
              as Map<String, dynamic>;
      final response = MonitoringListResponse.fromJson(sharedResponse);

      expect(response.total, 2);
      expect(
        response.households
            .map((row) => [row.householdOccupancyId, row.houseNumber])
            .toList(),
        [
          ['occupancy-a', 'A-01'],
          ['occupancy-b', 'B-07'],
        ],
      );
      expect(sharedResponse['data'][0], isNot(contains('outstanding')));
      expect(sharedResponse['data'][0], isNot(contains('bill_status')));
      expect(sharedResponse['data'][0], isNot(contains('period')));
    },
  );

  test('parses backend data list and keeps summary consistent', () {
    final response = MonitoringListResponse.fromJson({
      'data': [
        {'household_occupancy_id': 'occupancy-1', 'house_number': 'A-01'},
      ],
      'total': 1,
      'eval_date': '2026-11-06',
    });

    expect(response.total, 1);
    expect(response.households, hasLength(1));
    expect(response.households.single.houseNumber, 'A-01');
  });

  test('parses an empty backend data list with zero total', () {
    final response = MonitoringListResponse.fromJson({
      'data': <Map<String, dynamic>>[],
      'total': 0,
      'eval_date': '2026-11-06',
    });

    expect(response.total, 0);
    expect(response.households, isEmpty);
  });
}
