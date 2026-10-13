import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';

void main() {
  final client = IuranApiClient(dio: Dio());

  Map<String, dynamic> bill(String id) => {
    'id': id,
    'rt_id': 'rt-1',
    'household_occupancy_id': 'occ-$id',
    'due_id': 'due-1',
    'amount': '50000.00',
    'period': '2026-10',
    'due_date': '2026-10-25',
    'status': 'unpaid',
  };

  test('parses generated_count and bills from backend response', () {
    final result = client.parseGenerateBillsResponse({
      'generated_count': 1,
      'bills': [bill('bill-1')],
    });

    expect(result.generatedCount, 1);
    expect(result.bills.single.id, 'bill-1');
  });

  test('accepts a valid empty bills response', () {
    final result = client.parseGenerateBillsResponse({
      'generated_count': 0,
      'bills': <dynamic>[],
    });

    expect(result.generatedCount, 0);
    expect(result.bills, isEmpty);
  });

  test('rejects an unknown response shape', () {
    expect(
      () => client.parseGenerateBillsResponse({'data': []}),
      throwsFormatException,
    );
  });

  test('rejects generated_count that disagrees with bills', () {
    expect(
      () => client.parseGenerateBillsResponse({
        'generated_count': 2,
        'bills': [bill('bill-1')],
      }),
      throwsFormatException,
    );
  });

  test('rejects malformed bill items', () {
    expect(
      () => client.parseGenerateBillsResponse({
        'generated_count': 1,
        'bills': ['not-a-bill'],
      }),
      throwsFormatException,
    );
  });
}
