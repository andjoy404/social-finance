import 'package:flutter/widgets.dart';

import 'iuran_screen.dart';

/// User-scoped Tagihan tab using the shared Iuran list presentation.
class IuranArrearsScreen extends StatelessWidget {
  const IuranArrearsScreen({super.key});

  @override
  Widget build(BuildContext context) => const IuranScreen(myBills: true);
}
