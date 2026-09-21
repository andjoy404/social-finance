import 'package:flutter/material.dart';
import '../../../../core/widgets/empty_state.dart';

class DuesScreen extends StatelessWidget {
  const DuesScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Iuran')),
      body: const Center(
        child: EmptyState(
          icon: Icons.receipt_long_outlined,
          title: 'Iuran',
          description:
              'Pengelolaan iuran warga akan tersedia pada tahap berikutnya.',
        ),
      ),
    );
  }
}
